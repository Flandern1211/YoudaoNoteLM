package ingestion

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/schema"
	milvusclient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

const (
	VectorDim = 2048 // 默认维度，实际由 embedder 决定
)

// UserCollectionName 返回用户专属的 Milvus Collection 名称
func UserCollectionName(userID uint) string {
	return fmt.Sprintf("user_%d_chunks", userID)
}

// MilvusIndexerConfig Milvus 连接配置
type MilvusIndexerConfig struct {
	Address string
}

// MilvusWriter 封装 Milvus 写入操作
type MilvusWriter struct {
	client milvusclient.Client
}

// NewMilvusWriter 创建 Milvus 写入器
func NewMilvusWriter(ctx context.Context, cfg MilvusIndexerConfig) (*MilvusWriter, error) {
	cli, err := milvusclient.NewClient(ctx, milvusclient.Config{
		Address: cfg.Address,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 Milvus 客户端失败: %w", err)
	}
	return &MilvusWriter{client: cli}, nil
}

// EnsureCollection 确保用户专属 Collection 存在，不存在则创建
func (w *MilvusWriter) EnsureCollection(ctx context.Context, userID uint) error {
	collName := UserCollectionName(userID)

	has, err := w.client.HasCollection(ctx, collName)
	if err != nil {
		return fmt.Errorf("检查 Collection 失败: %w", err)
	}
	if has {
		return nil
	}

	schema := &entity.Schema{
		CollectionName: collName,
		AutoID:         true,
		Fields: []*entity.Field{
			{
				Name:       "id",
				DataType:   entity.FieldTypeInt64,
				AutoID:     true,
				PrimaryKey: true,
			},
			{
				Name:     "content",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					"max_length": "8192",
				},
			},
			{
				Name:     "vector",
				DataType: entity.FieldTypeFloatVector,
				TypeParams: map[string]string{
					"dim": fmt.Sprintf("%d", VectorDim),
				},
			},
			{
				Name:     "parent_block_id",
				DataType: entity.FieldTypeInt64,
			},
			{
				Name:     "source_id",
				DataType: entity.FieldTypeInt64,
			},
			{
				Name:     "chunk_type",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					"max_length": "32",
				},
			},
			{
				Name:     "metadata",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					"max_length": "2048",
				},
			},
		},
	}

	if err := w.client.CreateCollection(ctx, schema, 2); err != nil {
		return fmt.Errorf("创建 Collection 失败: %w", err)
	}

	// 创建 HNSW 索引
	idxParam, err := entity.NewIndexHNSW(entity.COSINE, 16, 200)
	if err != nil {
		return fmt.Errorf("创建 HNSW 索引参数失败: %w", err)
	}
	if err := w.client.CreateIndex(ctx, collName, "vector", idxParam, false); err != nil {
		return fmt.Errorf("创建索引失败: %w", err)
	}

	// 加载 Collection
	if err := w.client.LoadCollection(ctx, collName, false); err != nil {
		return fmt.Errorf("加载 Collection 失败: %w", err)
	}

	return nil
}

// Store 将文档和对应的向量写入用户专属 Collection
func (w *MilvusWriter) Store(ctx context.Context, userID uint, docs []*schema.Document, vectors [][]float32) error {
	if len(docs) == 0 || len(docs) != len(vectors) {
		return fmt.Errorf("文档和向量数量不匹配: docs=%d, vectors=%d", len(docs), len(vectors))
	}

	n := len(docs)
	contents := make([]string, n)
	vectorsData := make([][]float32, n)
	parentBlockIDs := make([]int64, n)
	sourceIDs := make([]int64, n)
	chunkTypes := make([]string, n)
	metadatas := make([]string, n)

	for i, doc := range docs {
		contents[i] = doc.Content
		vectorsData[i] = vectors[i]

		if pid, ok := doc.MetaData["parent_index"].(int); ok {
			parentBlockIDs[i] = int64(pid)
		}
		if sid, ok := doc.MetaData["source_id"].(uint); ok {
			sourceIDs[i] = int64(sid)
		}
		if ct, ok := doc.MetaData["chunk_type"].(string); ok {
			chunkTypes[i] = ct
		}
		metaJSON, _ := json.Marshal(doc.MetaData)
		metadatas[i] = string(metaJSON)
	}

	// 按 batch 写入
	batchSize := 100
	for start := 0; start < n; start += batchSize {
		end := start + batchSize
		if end > n {
			end = n
		}

		columns := []entity.Column{
			entity.NewColumnVarChar("content", contents[start:end]),
			entity.NewColumnFloatVector("vector", VectorDim, vectorsData[start:end]),
			entity.NewColumnInt64("parent_block_id", parentBlockIDs[start:end]),
			entity.NewColumnInt64("source_id", sourceIDs[start:end]),
			entity.NewColumnVarChar("chunk_type", chunkTypes[start:end]),
			entity.NewColumnVarChar("metadata", metadatas[start:end]),
		}

		if _, err := w.client.Insert(ctx, UserCollectionName(userID), "", columns...); err != nil {
			return fmt.Errorf("写入 Milvus 失败 (batch %d-%d): %w", start, end, err)
		}
	}

	return nil
}

// DeleteBySourceID 删除指定 source 在用户专属 Collection 中的所有 ChildChunk
func (w *MilvusWriter) DeleteBySourceID(ctx context.Context, userID uint, sourceID uint) error {
	expr := fmt.Sprintf(`source_id == %d`, sourceID)
	return w.client.Delete(ctx, UserCollectionName(userID), "", expr)
}

// Close 关闭 Milvus 客户端
func (w *MilvusWriter) Close() {
	w.client.Close()
}

// WrapDocuments 为 ChildChunk Document 添加 source_id 元数据
func WrapDocuments(docs []*schema.Document, sourceID uint) []*schema.Document {
	for _, doc := range docs {
		doc.MetaData["source_id"] = sourceID
	}
	return docs
}
