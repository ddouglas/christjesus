package server

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5"
)

func (s *Service) handleGetAdminNeedDocument(w http.ResponseWriter, r *http.Request) {
	needID := strings.TrimSpace(r.PathValue("id"))
	documentID := strings.TrimSpace(r.PathValue("documentID"))
	if needID == "" || documentID == "" {
		http.NotFound(w, r)
		return
	}

	doc, err := s.documentRepo.DocumentByNeedIDAndID(r.Context(), needID, documentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}

		s.logger.WithError(err).
			WithField("need_id", needID).
			WithField("document_id", documentID).
			Error("failed to fetch admin document record")
		s.internalServerError(w)
		return
	}

	storageKey := strings.TrimSpace(doc.StorageKey)
	if storageKey == "" {
		s.logger.WithField("need_id", needID).
			WithField("document_id", documentID).
			Error("admin document record missing storage key")
		s.internalServerError(w)
		return
	}

	response, err := s.s3Client.GetObject(r.Context(), &s3.GetObjectInput{
		Bucket: aws.String(s.config.S3BucketName),
		Key:    aws.String(storageKey),
	})
	if err != nil {
		if isS3NotFoundError(err) {
			http.NotFound(w, r)
			return
		}

		s.logger.WithError(err).
			WithField("need_id", needID).
			WithField("document_id", documentID).
			WithField("storage_key", storageKey).
			Error("failed to fetch admin document from s3")
		s.internalServerError(w)
		return
	}
	defer response.Body.Close()

	contentType := strings.TrimSpace(doc.MimeType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", safeContentDispositionFilename(doc.FileName, doc.ID)))
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, response.Body); err != nil {
		s.logger.WithError(err).
			WithField("need_id", needID).
			WithField("document_id", documentID).
			Warn("failed to stream admin document response")
	}
}

func safeContentDispositionFilename(fileName, fallbackID string) string {
	clean := strings.TrimSpace(fileName)
	if clean == "" {
		return "need-document-" + strings.TrimSpace(fallbackID)
	}

	replacer := strings.NewReplacer("\n", "", "\r", "", "\"", "")
	clean = strings.TrimSpace(replacer.Replace(clean))
	if clean == "" {
		return "need-document-" + strings.TrimSpace(fallbackID)
	}

	return clean
}
