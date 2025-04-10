package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

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
	defer file.Close()
	defer os.Remove("/tmp/tubely-upload.mp4")

	_, err = io.Copy(file, video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError,
			"Unable to save video on server", err)
		return
	}

	file.Seek(0, io.SeekStart)

	filename := getAssetPath(mediaType)
    _, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &filename,
		Body:        file,
		ContentType: &mth,
	})
    if err != nil {
        respondWithError(w, http.StatusInternalServerError,
            "cannot upload video", err)
        return
    }

    url := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", 
        cfg.s3Bucket, cfg.s3Region, filename)

    videoMetadata.VideoURL = &url
    err = cfg.db.UpdateVideo(videoMetadata)
    if err != nil {
        respondWithError(w, http.StatusInternalServerError,
            "Unable to update video", err)
        return
    }
}
