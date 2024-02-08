use std::fmt::Debug;
use std::io::Cursor;
use std::path::Path;

use anyhow::{anyhow, Context};
use async_trait::async_trait;
use mail_parser::{Message, MessageParser, MessagePartId, MimeHeaders};
use tempfile::{NamedTempFile, TempPath};

use identify::deduplication::dedupe_checksum;

use crate::mimetype;
use crate::processing::{Process, ProcessContext, ProcessOutput};

#[derive(Debug, Default)]
pub struct Rfc822EmbeddedProcessor {
    message_parser: MessageParser,
}

impl Rfc822EmbeddedProcessor {
    async fn process_part(
        &self,
        ctx: &ProcessContext,
        message: &Message<'_>,
        part_id: &MessagePartId
    ) -> Result<ProcessOutput, anyhow::Error> {
        let part = message
            .part(*part_id)
            .ok_or(anyhow!("failed to get attachment part"))?;
        let content_type = part
            .content_type()
            .ok_or(anyhow!("failed to get attachment content type"))?;
        let mimetype = mimetype(content_type);

        let mut reader = Cursor::new(part.contents());
        let checksum = dedupe_checksum(&mut reader, &mimetype).await?;
        let name = part.attachment_name().unwrap_or("message-attachment.dat");

        let mut file = NamedTempFile::new()?;
        std::io::copy(&mut part.contents(), &mut file)?;

        Ok(ProcessOutput::embedded(&ctx, name, file.into_temp_path(), mimetype, checksum))
    }
}

#[async_trait]
impl Process for Rfc822EmbeddedProcessor {
    async fn process(
        &self,
        ctx: ProcessContext,
        input_path: &Path,
        _: TempPath,
        _: &str,
    ) -> Result<(), anyhow::Error> {
        let content = std::fs::read(input_path)
            .context("failed to read input file")?;

        let message = self.message_parser.parse(&content)
            .context("failed to parse message")?;

        for part_id in &message.attachments {
            ctx.add_output(self.process_part(&ctx, &message, part_id).await).await?;
        }

        Ok(())
    }

    fn name(&self) -> &'static str {
        "RFC 822 Embedded"
    }
}
