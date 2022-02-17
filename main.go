package main

import (
	"archive/zip"
	"embed"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func main() {
	g := gin.Default()
	g.SetTrustedProxies([]string{"127.0.0.1"})

	g.GET("*path", handleListFiles)

	g.Run()
}

//go:embed templates/*
var templatesFS embed.FS
var templates = template.Must(template.ParseFS(templatesFS, "**/*.html"))

func handleListFiles(ctx *gin.Context) {
	path := ctx.Param("path")
	realPath := filepath.Join("wwwroot", path)

	fInfo, err := os.Stat(realPath)
	if err != nil {
		ctx.AbortWithError(500, err)
		return
	}

	if !fInfo.IsDir() {
		ctx.File(realPath)
		return
	}

	files, err := os.ReadDir(realPath)
	if err != nil {
		ctx.AbortWithError(500, err)
		return
	}

	if _, ok := ctx.GetQuery("zip"); ok {
		z := zip.NewWriter(ctx.Writer)

		ctx.Status(200)
		ctx.Header("Content-Type", "application/zip")
		if err := filepath.Walk(realPath, func(filePath string, info os.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}

			if err != nil {
				return err
			}

			relPath := strings.TrimPrefix(filePath, realPath+"\\")
			fmt.Println(relPath)
			zipFile, err := z.Create(relPath)
			if err != nil {
				return err
			}
			fsFile, err := os.Open(filePath)
			if err != nil {
				return err
			}
			_, err = io.Copy(zipFile, fsFile)
			if err != nil {
				return err
			}
			return nil
		}); err != nil {
			ctx.AbortWithError(500, err)
			return
		}
		z.Close()
		return
	}

	type fileListing struct {
		Path  string `json:"path"`
		Name  string `json:"name"`
		IsDir bool   `json:"isDir"`
		Size  string `json:"size,omitempty"`
	}
	var fileListings []fileListing

	for _, f := range files {
		info, err := f.Info()
		if err != nil {
			ctx.AbortWithError(500, err)
			return
		}
		var slashIfDir string
		if info.IsDir() {
			slashIfDir = "/"
		}
		fileListings = append(fileListings, fileListing{
			Path:  strings.ReplaceAll(filepath.Join(path, f.Name()), "\\", "/"),
			Name:  f.Name() + slashIfDir,
			IsDir: f.IsDir(),
			Size:  HumanFileSize(info.Size()),
		})
	}

	ctx.Status(200)
	ctx.Header("Content-Type", "text/html")
	templates.ExecuteTemplate(ctx.Writer, "filelisting.html", map[string]interface{}{
		"Dir":   path,
		"Files": fileListings,
	})
}

func HumanFileSize(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}
