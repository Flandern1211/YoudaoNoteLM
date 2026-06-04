package service

import (
	bizerrors "YoudaoNoteLm/pkg/errors"
	"YoudaoNoteLm/pkg/utils"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand"
	"time"

	dto "YoudaoNoteLm/internal/model/dto/response"

	"github.com/redis/go-redis/v9"
)

const (
	captchaTTL       = 5 * time.Minute // 验证码有效期 5 分钟
	captchaWidth     = 300             // 背景图宽度
	captchaHeight    = 150             // 背景图高度
	sliderSize       = 40              // 滑块大小
	captchaTolerance = 5               // 容差像素
)

// captchaService 验证码服务实现
type captchaService struct {
	redis *redis.Client
}

// NewCaptchaService 创建验证码服务
func NewCaptchaService(redisClient *redis.Client) CaptchaService {
	return &captchaService{redis: redisClient}
}

// captchaKey Redis key
func (s *captchaService) captchaKey(captchaID string) string {
	return fmt.Sprintf("captcha:%s", captchaID)
}

// Generate 生成滑块验证码
func (s *captchaService) Generate(ctx context.Context) (*dto.CaptchaData, error) {
	// 生成验证码 ID
	captchaID, err := utils.GenerateRandomString(16)
	if err != nil {
		return nil, fmt.Errorf("生成验证码ID失败: %w", err)
	}

	// 随机生成滑块 X 位置（留边距）
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	sliderX := sliderSize + r.Intn(captchaWidth-3*sliderSize)
	sliderY := sliderSize + r.Intn(captchaHeight-2*sliderSize)

	// 生成背景图
	bgImg := generateBackground(captchaWidth, captchaHeight, sliderX, sliderY)

	// 生成滑块图（从背景图裁切）
	sliderImg := generateSlider(bgImg, sliderX, sliderY)

	// 编码为 base64
	bgBase64, err := imageToBase64(bgImg)
	if err != nil {
		return nil, err
	}
	sliderBase64, err := imageToBase64(sliderImg)
	if err != nil {
		return nil, err
	}

	// 存储正确位置到 Redis
	if err := s.redis.Set(ctx, s.captchaKey(captchaID), sliderX, captchaTTL).Err(); err != nil {
		return nil, fmt.Errorf("存储验证码失败: %w", err)
	}

	return &dto.CaptchaData{
		CaptchaID:    captchaID,
		Background:   bgBase64,
		Slider:       sliderBase64,
		SliderSize:   sliderSize,
		BgWidth:      captchaWidth,
		SliderStartX: 0,
	}, nil
}

// Verify 校验滑块验证码
func (s *captchaService) Verify(ctx context.Context, captchaID string, userX int) error {
	key := s.captchaKey(captchaID)

	// 获取正确位置
	correctX, err := s.redis.Get(ctx, key).Int()
	if err == redis.Nil {
		return bizerrors.New(bizerrors.CodeInvalidParam, "验证码已过期，请重新获取")
	}
	if err != nil {
		return fmt.Errorf("查询验证码失败: %w", err)
	}

	// 删除验证码（一次性使用）
	s.redis.Del(ctx, key)

	// 校验位置（允许误差）
	if math.Abs(float64(userX-correctX)) > captchaTolerance {
		return bizerrors.New(bizerrors.CodeInvalidParam, "滑块验证失败，请重试")
	}

	return nil
}

// generateBackground 生成背景图（带滑块凹槽）
func generateBackground(width, height, slotX, slotY int) *image.RGBA {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// 绘制渐变背景
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// 蓝灰色渐变 + 随机噪点
			noise := r.Intn(20) - 10
			baseR := uint8(100 + noise)
			baseG := uint8(130 + noise)
			baseB := uint8(180 + noise + (x*30)/width)
			img.Set(x, y, color.RGBA{R: baseR, G: baseG, B: baseB, A: 255})
		}
	}

	// 绘制凹槽（深色阴影）
	for y := slotY; y < slotY+sliderSize; y++ {
		for x := slotX; x < slotX+sliderSize; x++ {
			if y < height && x < width {
				// 凹槽边缘渐变
				distX := min(x-slotX, slotX+sliderSize-1-x)
				distY := min(y-slotY, slotY+sliderSize-1-y)
				dist := min(distX, distY)
				alpha := uint8(80)
				if dist < 3 {
					alpha = uint8(40 + dist*15)
				}
				orig := img.RGBAAt(x, y)
				shadowR := uint8(int(orig.R) * 60 / 100)
				shadowG := uint8(int(orig.G) * 60 / 100)
				shadowB := uint8(int(orig.B) * 60 / 100)
				img.Set(x, y, color.RGBA{R: shadowR, G: shadowG, B: shadowB, A: alpha})
			}
		}
	}

	// 绘制干扰线
	for i := 0; i < 5; i++ {
		x1 := r.Intn(width)
		y1 := r.Intn(height)
		x2 := r.Intn(width)
		y2 := r.Intn(height)
		drawLine(img, x1, y1, x2, y2, color.RGBA{R: 200, G: 200, B: 200, A: 80})
	}

	return img
}

// generateSlider 生成滑块图
func generateSlider(bg *image.RGBA, slotX, slotY int) *image.RGBA {
	slider := image.NewRGBA(image.Rect(0, 0, sliderSize, sliderSize))

	// 从背景图复制滑块区域
	for y := 0; y < sliderSize; y++ {
		for x := 0; x < sliderSize; x++ {
			srcX := slotX + x
			srcY := slotY + y
			if srcX < bg.Bounds().Dx() && srcY < bg.Bounds().Dy() {
				slider.Set(x, y, bg.RGBAAt(srcX, srcY))
			}
		}
	}

	// 绘制滑块边框
	borderColor := color.RGBA{R: 255, G: 255, B: 255, A: 200}
	for i := 0; i < sliderSize; i++ {
		slider.Set(i, 0, borderColor)
		slider.Set(i, sliderSize-1, borderColor)
		slider.Set(0, i, borderColor)
		slider.Set(sliderSize-1, i, borderColor)
	}

	return slider
}

// drawLine 画线
func drawLine(img *image.RGBA, x1, y1, x2, y2 int, c color.RGBA) {
	dx := abs(x2 - x1)
	dy := abs(y2 - y1)
	sx, sy := 1, 1
	if x1 >= x2 {
		sx = -1
	}
	if y1 >= y2 {
		sy = -1
	}
	err := dx - dy
	for {
		if x1 >= 0 && x1 < img.Bounds().Dx() && y1 >= 0 && y1 < img.Bounds().Dy() {
			img.Set(x1, y1, c)
		}
		if x1 == x2 && y1 == y2 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x1 += sx
		}
		if e2 < dx {
			err += dx
			y1 += sy
		}
	}
}

// imageToBase64 图片转 base64
func imageToBase64(img image.Image) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("编码图片失败: %w", err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
