package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const uploadLimit = 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, uploadLimit)

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
	videoMetaData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Could not find video metadata for that videoID", err)
		return
	}
	if videoMetaData.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorized to update video", err)
		return
	}

	videoFile, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse video form file", err)
		return
	}
	defer videoFile.Close()

	contentType := header.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse mediaType from request header", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid media type, expect mp4", nil)
		return
	}

	fileReference, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error writing the video file", err)
		return
	}
	defer os.Remove(fileReference.Name())
	defer fileReference.Close()

	_, err = io.Copy(fileReference, videoFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error writing the video file", err)
		return
	}

	_, err = fileReference.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable reset fileReference pointer", err)
		return
	}

	aspectRatio, err := getVideoAspectRatio(fileReference.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to retrieve apect ratio from vidoe", err)
		return
	}
	var ratioPrefix string
	switch aspectRatio {
	case "16:9":
		ratioPrefix = "landscape"
	case "9:16":
		ratioPrefix = "portrait"
	default:
		ratioPrefix = "other"
	}

	fastStartFile, err := processVideoForFastStart(fileReference.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to process Video For Fast Start", err)
		return
	}
	fastStartFileReference, err := os.Open(fastStartFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to open Fast Start output video", err)
		return
	}
	defer os.Remove(fastStartFileReference.Name())
	defer fastStartFileReference.Close()

	videoFileExtension := strings.Replace(mediaType, "video/", "", 1)
	randomName := make([]byte, 32)
	rand.Read(randomName)
	randomVideoURL := base64.RawURLEncoding.EncodeToString(randomName)

	key := fmt.Sprintf("%s/%s.%s", ratioPrefix, randomVideoURL, videoFileExtension)

	putParams := s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(key),
		Body:        fastStartFileReference,
		ContentType: aws.String(mediaType),
	}
	_, err = cfg.s3Client.PutObject(r.Context(), &putParams)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to upload the video to the s3 bucket", err)
		return
	}

	cloudfrontVideoURL := fmt.Sprintf("%s/%s", cfg.s3CfDistribution, key)

	videoMetaData.VideoURL = &cloudfrontVideoURL

	err = cfg.db.UpdateVideo(videoMetaData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update video url in database", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoMetaData)
}
