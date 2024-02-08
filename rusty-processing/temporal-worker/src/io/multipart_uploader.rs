use std::path::Path;

use aws_sdk_s3::primitives::ByteStream;
use aws_sdk_s3::types::{CompletedMultipartUpload, CompletedPart};
use bytesize::MB;
use tokio::io::{AsyncRead, AsyncReadExt};

use crate::s3_client;
use crate::util::parse_s3_uri;

pub struct MultipartUploader {
    bucket: String,
    key: String,
}

impl MultipartUploader {
    pub fn new(s3_uri: impl AsRef<Path>) -> anyhow::Result<Self> {
        let (bucket, key) = parse_s3_uri(s3_uri.as_ref())?;
        Ok(Self { bucket, key })
    }

    pub async fn upload(&self, reader: &mut (dyn AsyncRead + Send + Sync + Unpin)) -> anyhow::Result<()> {
        let multipart_upload = s3_client()
            .await
            .create_multipart_upload()
            .bucket(&self.bucket)
            .key(&self.key)
            .send()
            .await?;

        if let Some(upload_id) = &multipart_upload.upload_id {
            let parts = self.upload_parts(upload_id, reader).await?;
            self.complete_upload(upload_id, parts).await?;
        }

        Ok(())
    }

    async fn upload_parts(
        &self,
        upload_id: impl AsRef<str>,
        reader: &mut (dyn AsyncRead + Send + Sync + Unpin),
    ) -> anyhow::Result<Vec<CompletedPart>> {
        let mut parts = vec![];
        let mut buf = Box::new([0; MB as usize]);
        let mut part_num = 1_i32;

        while let Ok(bytes_read) = reader.read(buf.as_mut()).await {
            if bytes_read == 0 {
                break;
            }

            let upload_part = s3_client()
                .await
                .upload_part()
                .bucket(&self.bucket)
                .key(&self.key)
                .upload_id(upload_id.as_ref())
                .body(ByteStream::from(buf.to_vec()))
                .part_number(part_num)
                .send()
                .await?;

            parts.push(
                CompletedPart::builder()
                    .e_tag(upload_part.e_tag.unwrap_or_default())
                    .part_number(part_num)
                    .build(),
            );

            part_num += 1;
        }

        Ok(parts)
    }

    async fn complete_upload(
        &self,
        upload_id: impl AsRef<str>,
        parts: Vec<CompletedPart>,
    ) -> anyhow::Result<()> {
        let completed_multipart_upload = CompletedMultipartUpload::builder()
            .set_parts(Some(parts))
            .build();

        s3_client()
            .await
            .complete_multipart_upload()
            .bucket(&self.bucket)
            .key(&self.key)
            .multipart_upload(completed_multipart_upload)
            .upload_id(upload_id.as_ref())
            .send()
            .await?;

        Ok(())
    }
}
