package store

import (
	"christjesus/pkg/types"
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5/pgxpool"
)

const documentTableName = "christjesus.need_documents"

var documentTableColumns = []string{
	"id",
	"need_id",
	"user_id",
	"document_type",
	"file_name",
	"file_size_bytes",
	"mime_type",
	"storage_key",
	"uploaded_at",
}

type DocumentRepository struct {
	pool *pgxpool.Pool
}

func NewDocumentRepository(pool *pgxpool.Pool) *DocumentRepository {
	return &DocumentRepository{pool: pool}
}

// GetDocumentByID retrieves a single document by ID
func (r *DocumentRepository) GetDocumentByID(ctx context.Context, id string) (*types.NeedDocument, error) {
	query, args, _ := psql().
		Select(documentTableColumns...).
		From(documentTableName).
		Where(squirrel.Eq{"id": id}).
		ToSql()

	var doc types.NeedDocument
	err := pgxscan.Get(ctx, r.pool, &doc, query, args...)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// GetDocumentsByNeedID retrieves all documents for a specific need
func (r *DocumentRepository) GetDocumentsByNeedID(ctx context.Context, needID string) ([]types.NeedDocument, error) {
	query, args, _ := psql().
		Select(documentTableColumns...).
		From(documentTableName).
		Where(squirrel.Eq{"need_id": needID}).
		OrderBy("uploaded_at DESC").
		ToSql()

	var docs []types.NeedDocument
	err := pgxscan.Select(ctx, r.pool, &docs, query, args...)
	if err != nil {
		return nil, err
	}
	return docs, nil
}

// CreateDocument inserts a new document record
func (r *DocumentRepository) CreateDocument(ctx context.Context, doc *types.NeedDocument) error {
	query, args, _ := psql().
		Insert(documentTableName).
		Columns(documentTableColumns...).
		Values(
			doc.ID,
			doc.NeedID,
			doc.UserID,
			doc.DocumentType,
			doc.FileName,
			doc.FileSizeBytes,
			doc.MimeType,
			doc.StorageKey,
			doc.UploadedAt,
		).
		ToSql()

	_, err := r.pool.Exec(ctx, query, args...)
	return err
}

// UpdateDocumentMetadata updates editable metadata fields for an existing document
func (r *DocumentRepository) UpdateDocumentMetadata(ctx context.Context, needID, id, fileName, documentType string) error {
	query, args, _ := psql().
		Update(documentTableName).
		Set("file_name", fileName).
		Set("document_type", documentType).
		Where(squirrel.Eq{"id": id, "need_id": needID}).
		ToSql()

	_, err := r.pool.Exec(ctx, query, args...)
	return err
}

// DeleteDocument removes a document record
func (r *DocumentRepository) DeleteDocument(ctx context.Context, id string) error {
	query, args, _ := psql().
		Delete(documentTableName).
		Where(squirrel.Eq{"id": id}).
		ToSql()

	_, err := r.pool.Exec(ctx, query, args...)
	return err
}

// DeleteDocumentsByNeedID removes all documents for a specific need
func (r *DocumentRepository) DeleteDocumentsByNeedID(ctx context.Context, needID string) error {
	query, args, _ := psql().
		Delete(documentTableName).
		Where(squirrel.Eq{"need_id": needID}).
		ToSql()

	_, err := r.pool.Exec(ctx, query, args...)
	return err
}
