use std::fmt::Debug;
use std::io;
use std::path::{Path, PathBuf};

use anyhow::Context;
use async_trait::async_trait;
use futures::future::try_join_all;
use lazy_static::lazy_static;
use serde::{Deserialize, Serialize};
use tempfile::{NamedTempFile, TempPath};

use identify::deduplication::dedupe_checksum_from_path;

use crate::processing::{ProcessContext, ProcessType};

lazy_static! {
    static ref PROCESSOR: Processor = Processor;
}

/// Returns a reference to the global processor instance.
///
pub fn processor() -> &'static Processor {
    &PROCESSOR
}

/// Process is a trait that defines the interface for process data from a file or as raw bytes.
///
/// Process implementations are required to be thread safe.
///
#[async_trait]
pub(crate) trait Process: Send + Sync {
    /// Process a stream of bytes.
    ///
    /// # Arguments
    ///
    /// * `ctx` - The context of the processing operation.
    /// * `input_path` - The path to the input file.
    /// * `output_path` - The path to the metadata.json file.
    ///
    async fn process(
        &self,
        ctx: ProcessContext,
        input_path: &Path,
        output_path: TempPath,
        checksum: &str,
    ) -> Result<(), anyhow::Error>;

    /// Returns the name of the processor.
    ///
    fn name(&self) -> &'static str;
}


/// Structure defining the core processor.
///
/// The processor is the core processor of the library and is responsible for
/// determining the correct processor to use for a given MIME type, and then
/// delegating to that processor.
///
#[derive(Debug, Default, PartialEq, PartialOrd, Eq, Ord, Hash, Serialize, Deserialize)]
pub struct Processor;

impl Processor {
    /// Processes a stream of data.
    ///
    /// This method will determine the correct processor to use for the given
    /// MIME type, and then delegate to that processor.
    ///
    /// # Arguments
    ///
    /// * `ctx` - Context of the processing operation.
    /// * `stream` - Stream of data in `bytes::Bytes` of the content to process.
    ///
    pub async fn process(
        &self,
        ctx: ProcessContext,
        input_path: PathBuf,
    ) -> Result<(), anyhow::Error> {
        let checksum = dedupe_checksum_from_path(&input_path, &ctx.mimetype).await
            .context("failed to calculate checksum")?;

        let mut futures = vec![];
        for processor in self.determine_processors(&ctx.mimetype, &ctx.types) {
            let inner_ctx = ctx.clone();
            let input_path_ref = &input_path;
            let checksum = &checksum;
            let output_path = temp_path().context("failed to create temporary file")?;

            futures.push(async move {
                processor.process(inner_ctx, input_path_ref, output_path, checksum).await
            });
        }

        try_join_all(futures).await.map(|_| ())
    }

    fn determine_processors(&self, mimetype: &str, types: &[ProcessType]) -> Vec<Box<dyn Process>> {
        let mut processors = vec![];

        if types.contains(&ProcessType::Text) {
            if let Some(processor) = self.text_processor(mimetype) {
                processors.push(processor);
            }
        }
        if types.contains(&ProcessType::Metadata) {
            if let Some(processor) = self.metadata_processor(mimetype) {
                processors.push(processor);
            }
        }
        if types.contains(&ProcessType::Pdf) {
            if let Some(processor) = self.pdf_processor(mimetype) {
                processors.push(processor);
            }
        }
        if types.contains(&ProcessType::Embedded) {
            if let Some(processor) = self.embedded_processor(mimetype) {
                processors.push(processor);
            }
        }

        processors
    }

    /// Find a processor to extract text based on the MIME type.
    ///
    fn text_processor(&self, mimetype: &str) -> Option<Box<dyn Process>> {
        match mimetype {
            "text/plain " |
            "text/css" |
            "text/csv" |
            "text/javascript" |
            "application/zip" |
            "application/mbox" => None,

            _ => Some(Box::<crate::text::DefaultTextProcessor>::default()),
        }
    }

    /// Find a processor to extract metadata based on the MIME type.
    ///
    fn metadata_processor(&self, _mimetype: &str) -> Option<Box<dyn Process>> {
        Some(Box::<crate::metadata::DefaultMetadataProcessor>::default())
    }

    /// Find a processor to render to a PDF based on the MIME type.
    ///
    fn pdf_processor(&self, mimetype: &str) -> Option<Box<dyn Process>> {
        match mimetype {
            "message/rfc822" => Some(Box::<crate::pdf::Rfc822PdfProcessor>::default()),

            _ => None
        }
    }

    /// Find a processor to extract embedded files based on the MIME type.
    ///
    fn embedded_processor(&self, mimetype: &str) -> Option<Box<dyn Process>> {
        match mimetype {
            "application/zip" => Some(Box::<crate::embedded::ZipEmbeddedProcessor>::default()),
            "application/mbox" => Some(Box::<crate::embedded::MboxEmbeddedProcessor>::default()),
            "message/rfc822" => Some(Box::<crate::embedded::Rfc822EmbeddedProcessor>::default()),

            _ => None
        }
    }
}

/// Creates a temporary file and returns its path.
///
#[inline]
fn temp_path() -> io::Result<TempPath> {
    Ok(NamedTempFile::new()?.into_temp_path())
}