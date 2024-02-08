use std::fmt::Debug;
use std::fs::File;
use std::path::Path;
use anyhow::Context;

use async_trait::async_trait;
use mail_parser::MessageParser;
use tempfile::TempPath;

use crate::processing::{Process, ProcessContext, ProcessOutput};

mod html_message_visitor;
mod message_formatter;
mod message_visitor;
mod transformer;

mod pdf;

#[derive(Debug, Default)]
pub struct Rfc822PdfProcessor {
    message_parser: MessageParser,
}

#[async_trait]
impl Process for Rfc822PdfProcessor {
    async fn process(
        &self,
        ctx: ProcessContext,
        input_path: &Path,
        output_path: TempPath,
        checksum: &str,
    ) -> Result<(), anyhow::Error> {
        let result = async {
            let content = std::fs::read(input_path)
                .context("failed to read input file")?;

            let message = self.message_parser.parse(&content)
                .context("failed to parse message")?;

            let mut writer = File::create(&output_path)
                .context("failed to create output file")?;

            self.render_pdf(&message, &mut writer).await
                .map(|_| ProcessOutput::processed(&ctx, "rendered.pdf", output_path, "application/pdf", checksum))
                .context("failed to render pdf")
        }.await;

        ctx.add_output(result).await
    }

    fn name(&self) -> &'static str {
        "RFC 822 PDF"
    }
}
