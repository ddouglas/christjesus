package types

import "time"

// NeedDocument represents a file upload supporting a need
type NeedDocument struct {
	ID            string    `db:"id" json:"id"`
	NeedID        string    `db:"need_id" json:"needId"`
	UserID        string    `db:"user_id" json:"userId"`
	DocumentType  string    `db:"document_type" json:"documentType"`
	FileName      string    `db:"file_name" json:"fileName"`
	FileSizeBytes int64     `db:"file_size_bytes" json:"fileSizeBytes"`
	MimeType      string    `db:"mime_type" json:"mimeType"`
	StorageKey    string    `db:"storage_key" json:"storageKey"`
	UploadedAt    time.Time `db:"uploaded_at" json:"uploadedAt"`
}

// Document type constants
const (
	DocTypeID                 = "id"
	DocTypeUtilityBill        = "utility_bill"
	DocTypeMedicalRecord      = "medical_record"
	DocTypeIncomeVerification = "income_verification"
	DocTypeEvictionNotice     = "eviction_notice"
	DocTypeOther              = "other"
)
