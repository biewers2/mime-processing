use std::io;
use std::io::{Read, Seek};
use std::path::Path;

use anyhow::{anyhow, Context};
use async_stream::stream;
use async_trait::async_trait;
use futures::{pin_mut, StreamExt};
use log::{debug, info, warn};
use serde::{Deserialize, Serialize};
use tempfile::{NamedTempFile, TempPath};
use zip::ZipArchive;

use identify::deduplication::dedupe_checksum_from_path;
use identify::mimetype::identify_mimetype;

use crate::processing::{Process, ProcessContext, ProcessOutput};

enum NextArchiveEntry {
    Dir(String),
    File(ArchiveEntry),
}

struct ArchiveEntry {
    name: String,
    path: TempPath,
    checksum: String,
    mimetype: String,
}

#[derive(Debug, Default, PartialEq, PartialOrd, Eq, Ord, Hash, Serialize, Deserialize)]
pub struct ZipEmbeddedProcessor;

#[async_trait]
impl Process for ZipEmbeddedProcessor {
    async fn process(
        &self,
        ctx: ProcessContext,
        path:&Path,
        _: TempPath,
        _: &str,
    ) -> Result<(), anyhow::Error> {
        info!("Opening zip file");
        let file = std::fs::File::open(path)
            .context("failed to open zip file")?;

        let reader = std::io::BufReader::new(file);
        let mut archive = ZipArchive::new(reader)
            .context("failed to open zip archive")?;

        info!("Streaming zip file entries");
        let output_stream = stream! {
            for i in 0..archive.len() {
                yield next_archive_entry(&mut archive, i).await;
            }
        };

        pin_mut!(output_stream);
        while let Some(result) = output_stream.next().await {
            match result {
                Ok(NextArchiveEntry::File(entry)) => {
                    info!("Discovered entry {}", entry.name);
                    let ArchiveEntry { name, path, checksum: dedupe_checksum, mimetype } = entry;
                    let output = ProcessOutput::embedded(&ctx, name, path, mimetype, dedupe_checksum);
                    ctx.add_output(Ok(output)).await?;
                },
                Ok(NextArchiveEntry::Dir(name)) => debug!("Discovered directory {}", name),
                Err(e) => {
                    warn!("Failed to read entry: {}", e);
                    ctx.add_output(Err(e)).await?;
                },
            }
        }

        Ok(())
    }

    fn name(&self) -> &'static str {
        "zip"
    }
}

async fn next_archive_entry<R>(archive: &mut ZipArchive<R>, index: usize) -> Result<NextArchiveEntry, anyhow::Error>
    where R: Read + Seek
{
    // Create an inner scope because `ZipFile` is not `Send` and must be dropped before `await`ing
    let (name, path) = {
        let mut zipfile = archive.by_index(index)
            .context("failed to read zip entry by index")?;

        let name = zipfile.enclosed_name()
            .and_then(|name| name.file_name())
            .map(|name| name.to_string_lossy().to_string())
            .ok_or(anyhow!("failed to get name for zip entry"))?;

        if zipfile.is_dir() {
            return Ok(NextArchiveEntry::Dir(name));
        }

        let emb_path = spool_read(&mut zipfile)?;
        (name, emb_path)
    };

    let mimetype = identify_mimetype(&path).await?.unwrap_or("application/octet-stream".to_string());
    let checksum = dedupe_checksum_from_path(&path, &mimetype).await?;

    Ok(NextArchiveEntry::File(ArchiveEntry { name, path, checksum, mimetype }))
}

/// Write contents to a temporary file and return the temporary path.
///
fn spool_read(mut reader: impl Read) -> io::Result<TempPath> {
    let mut file = NamedTempFile::new()?;
    std::io::copy(&mut reader, &mut file)?;
    Ok(file.into_temp_path())
}