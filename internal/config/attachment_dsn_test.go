package config

import "testing"

func TestParseAttachmentDSN(t *testing.T) {
	local := ParseAttachmentDSN("file://./data/attachments")
	if local.Kind != AttachmentLocal || local.LocalPath != "./data/attachments" {
		t.Fatalf("local: %+v", local)
	}

	s3 := ParseAttachmentDSN("s3://rustfsadmin:rustfsadmin@localhost:9000/mcphub/attachments?path_style=true")
	if s3.Kind != AttachmentS3 {
		t.Fatalf("kind: %s", s3.Kind)
	}
	if s3.S3.Bucket != "mcphub" || s3.S3.Prefix != "attachments" {
		t.Fatalf("bucket/prefix: %+v", s3.S3)
	}
	if s3.S3.AccessKey != "rustfsadmin" || !s3.S3.PathStyle {
		t.Fatalf("creds/path_style: %+v", s3.S3)
	}
}
