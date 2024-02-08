use std::path::Path;

use anyhow::Context;
use async_trait::async_trait;
use tempfile::TempPath;

use services::tika;

use crate::processing::{Process, ProcessContext, ProcessOutput};

#[derive(Debug, Default, Clone, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub struct DefaultMetadataProcessor;

#[async_trait]
impl Process for DefaultMetadataProcessor {
    async fn process(
        &self,
        ctx: ProcessContext,
        input_path: &Path,
        output_path: TempPath,
        checksum: &str,
    ) -> Result<(), anyhow::Error> {
        let result = async {
            let mut metadata = tika().metadata(input_path).await
                .context("failed to extract metadata")?;

            tokio::fs::write(&output_path, &mut metadata).await
                .context("failed to write metadata to file")?;

            Ok(ProcessOutput::processed(&ctx, "metadata.json", output_path, "application/json", checksum))
        }.await;

        ctx.add_output(result).await
    }

    fn name(&self) -> &'static str {
        "Default Metadata"
    }
}