package database

import "context"

// bgCtx is used for the handful of MinIO admin calls made outside of a
// request (bucket setup at boot). Per-request S3 presigning goes through
// nex-protocols-common-go's own S3Presigner, which manages its own context.
var bgCtx = context.Background()
