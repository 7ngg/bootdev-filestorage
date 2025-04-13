package main

import (
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println("uploading video", videoID, "by user", userID)

	const maxMemory = 1 << 30
	r.ParseMultipartForm(maxMemory)

	videoMetadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Video not found", err)
		return
	}

	if userID != videoMetadata.UserID {
		respondWithError(
			w, http.StatusUnauthorized,
			"Not authorized to edit this video",
			err)
		return
	}

	video, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(
			w, http.StatusBadRequest,
			"Unable to parse for data", err)
		return
	}
	defer video.Close()

	mth := header.Header.Get("Content-Type")
	if mth == "" {
		respondWithError(w, http.StatusBadRequest,
			"Missing Content-Type for video", err)
	}
	mediaType, _, err := mime.ParseMediaType(mth)
	if mediaType != "video/mp4" {
		respondWithError(
			w, http.StatusBadRequest,
			"Unsupported media type", err)
		return
	}

	file, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"Unable to create file on server", err)
		return
	}
	defer os.Remove(file.Name())
	defer file.Close()

	if _, err = io.Copy(file, video); err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"Unable to save video on server", err)
		return
	}

	if _, err = file.Seek(0, io.SeekStart); err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"Unable to reset file pointer", err)
		return
	}

	fastStartVideoPath, err := processVideoForFastStart(file.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"Unable to process video for fasr start", err)
		return
	}
	defer os.Remove(fastStartVideoPath)

	fastStartVideo, err := os.Open(fastStartVideoPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"Unable to open fast start video", err)
		return
	}
	defer fastStartVideo.Close()

	aspectRatio, _ := getVideoAspectRatio(fastStartVideoPath)
	key := filepath.Join(aspectRatio, getAssetPath(mediaType))
	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &key,
		Body:        fastStartVideo,
		ContentType: &mth,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"cannot upload video", err)
		return
	}

	url := cfg.s3CfDistribution + "/" + key
    log.Println(url)
	videoMetadata.VideoURL = &url
	err = cfg.db.UpdateVideo(videoMetadata)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"Unable to update video", err)
		return
	}

    respondWithJSON(w, http.StatusOK, videoMetadata)
}
