package database

import (
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"lost-reavers/globals"
)

// MinIOClient backs DataStore object bodies (treasure/inventory blobs). It is
// nil until ConnectS3 runs, which init.go calls right after Connect.
var MinIOClient *minio.Client

// ConnectS3 opens the S3-compatible client used for DataStore object storage
// and makes sure the configured bucket exists. Metadata (name, size, owner,
// permissions, ...) lives in Postgres; only the object body lives in S3 -- see
// database/datastore.go.
func ConnectS3() error {
	client, err := minio.New(globals.Settings.S3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(globals.Settings.S3AccessKey, globals.Settings.S3SecretKey, ""),
		Secure: globals.Settings.S3UseSSL,
		Region: globals.Settings.S3Region,
	})
	if err != nil {
		return fmt.Errorf("could not create the S3 client: %w", err)
	}

	MinIOClient = client

	exists, err := client.BucketExists(bgCtx, globals.Settings.S3Bucket)
	if err != nil {
		return fmt.Errorf("could not reach the S3 endpoint %s: %w", globals.Settings.S3Endpoint, err)
	}

	if !exists {
		if err := client.MakeBucket(bgCtx, globals.Settings.S3Bucket, minio.MakeBucketOptions{Region: globals.Settings.S3Region}); err != nil {
			return fmt.Errorf("could not create S3 bucket %s: %w", globals.Settings.S3Bucket, err)
		}

		globals.Logger.Successf("Created S3 bucket %s", globals.Settings.S3Bucket)
	}

	globals.Logger.Successf("Connected to S3 storage at %s (bucket %s)", globals.Settings.S3Endpoint, globals.Settings.S3Bucket)

	return nil
}
