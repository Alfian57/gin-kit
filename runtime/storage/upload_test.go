package storage

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSaveUploadStoresMultipartFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	disk := testLocal(t, "")

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("avatar", "photo.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("png-bytes")); err != nil {
		t.Fatal(err)
	}
	writer.Close()

	router := gin.New()
	router.POST("/upload", func(c *gin.Context) {
		size, err := SaveUpload(c, disk, "avatar", "avatars/user-1.png")
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.String(http.StatusOK, "%d", size)
	})
	request := httptest.NewRequest(http.MethodPost, "/upload", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "9" {
		t.Fatalf("upload failed: %d %s", recorder.Code, recorder.Body)
	}
	reader, err := disk.Get(context.Background(), "avatars/user-1.png")
	if err != nil {
		t.Fatal(err)
	}
	stored, _ := io.ReadAll(reader)
	reader.Close()
	if string(stored) != "png-bytes" {
		t.Fatalf("stored content mismatch: %q", stored)
	}
}

func TestSaveUploadMissingFieldFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	disk := testLocal(t, "")
	router := gin.New()
	router.POST("/upload", func(c *gin.Context) {
		if _, err := SaveUpload(c, disk, "absent", "x"); err != nil {
			c.String(http.StatusBadRequest, "missing")
			return
		}
		c.Status(http.StatusOK)
	})
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/upload", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("missing field not rejected: %d", recorder.Code)
	}
}
