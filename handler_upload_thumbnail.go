package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	imageFile, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable parse form file", err)
		return
	}

	contentType := header.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	// ignoring the map to the original mediatype formatting (if it contained spaces or caps)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse mediaType from request header", err)
		return
	}
	if (mediaType != "image/jpeg") && (mediaType != "image/png") {
		respondWithError(w, http.StatusBadRequest, "Invalid Media type, expects png or jpeg", nil)
		return
	}

	videoMetaData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Video data not found", err)
		return
	}

	if videoMetaData.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorized to accces video", nil)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	fileExtension := strings.Replace(mediaType, "image/", "", 1)

	randomName := make([]byte, 32)
	rand.Read(randomName)

	randomVideoURL := base64.RawURLEncoding.EncodeToString(randomName)

	fullFileName := fmt.Sprintf("%s.%s", randomVideoURL, fileExtension)
	ImageFilePath := filepath.Join(cfg.assetsRoot, fullFileName)

	fileReference, err := os.Create(ImageFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error writing the image file", err)
		return
	}
	_, err = io.Copy(fileReference, imageFile)
	// ignoring the returned amount of bytes copied
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error writing the image file", err)
		return
	}

	thumbnailURL := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, fullFileName)

	videoParam := database.CreateVideoParams{
		Title:       videoMetaData.Title,
		Description: videoMetaData.Description,
		UserID:      videoMetaData.UserID,
	}

	updatedVideo := database.Video{
		ID:                videoMetaData.ID,
		CreatedAt:         videoMetaData.CreatedAt,
		UpdatedAt:         time.Now(),
		ThumbnailURL:      &thumbnailURL,
		VideoURL:          videoMetaData.VideoURL,
		CreateVideoParams: videoParam,
	}
	err = cfg.db.UpdateVideo(updatedVideo)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update the video with new thumbnail url in database", err)
		return
	}

	respondWithJSON(w, http.StatusOK, updatedVideo)
}
