package storage

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

// SaveUpload stores the multipart file from the named form field onto the
// disk at destPath, returning the stored size in bytes. The upload's declared
// content type is forwarded to the disk.
func SaveUpload(c *gin.Context, disk Disk, field, destPath string) (int64, error) {
	header, err := c.FormFile(field)
	if err != nil {
		return 0, fmt.Errorf("storage: read upload %q: %w", field, err)
	}
	file, err := header.Open()
	if err != nil {
		return 0, fmt.Errorf("storage: open upload %q: %w", field, err)
	}
	defer file.Close()
	err = disk.Put(c.Request.Context(), destPath, file,
		WithContentType(header.Header.Get("Content-Type")),
		WithSize(header.Size),
	)
	if err != nil {
		return 0, err
	}
	return header.Size, nil
}
