use std::fmt::Debug;
use std::io::{Cursor, Write};
use std::path::Path;

use anyhow::{anyhow, Context};
use async_trait::async_trait;
use log::info;
use mail_parser::mailbox::mbox::{Message, MessageIterator};
use serde::{Deserialize, Serialize};
use tempfile::{NamedTempFile, TempPath};

use identify::deduplication::dedupe_checksum;

use crate::processing::{Process, ProcessContext, ProcessOutput};

/// MboxProcessor is responsible for processing mbox files.
///
/// Internally it uses the `mail_parser` crate to parse the mbox file.
/// The processor only writes out embedded messages and doesn't produce any processed metadata.json.
///
#[derive(Debug, Default, PartialEq, PartialOrd, Eq, Ord, Hash, Serialize, Deserialize)]
pub struct MboxEmbeddedProcessor;

impl MboxEmbeddedProcessor {
    /// Writes a message to the metadata.json directory.
    ///
    async fn process_message(&self, ctx: &ProcessContext, message: Message) -> Result<ProcessOutput, anyhow::Error> {
        let mut file = NamedTempFile::new()
            .context("failed to create temporary file")?;

        let contents = message.unwrap_contents();
        file.write_all(&contents)
            .context("failed to write message to temporary file")?;

        let mimetype = "message/rfc822";
        let ctx = ctx.new_clone(mimetype.to_string());

        let mut contents = Cursor::new(contents);
        let checksum = dedupe_checksum(&mut contents, &mimetype).await
            .context("failed to calculate checksum")?;

        Ok(ProcessOutput::embedded(
            &ctx,
            "mbox-message.eml",
            file.into_temp_path(),
            mimetype,
            checksum,
        ))
    }
}

#[async_trait]
impl Process for MboxEmbeddedProcessor {
    async fn process(
        &self,
        ctx: ProcessContext,
        input_path: &Path,
        _: TempPath,
        _: &str,
    ) -> Result<(), anyhow::Error> {
        info!("Reading mbox into iterator");
        let file = std::fs::File::open(input_path)
            .context("failed to open mbox file")?;

        let reader = std::io::BufReader::new(file);
        let message_iter = MessageIterator::new(reader);

        info!("Processing embedded messages");
        for message_res in message_iter {
            let message_res = message_res.map_err(|_| anyhow!("failed to parse message from mbox"));
            match message_res {
                Ok(message) => ctx.add_output(self.process_message(&ctx, message).await).await?,
                Err(e) => ctx.add_output(Err(e)).await?,
            }
        }
        Ok(())
    }

    fn name(&self) -> &'static str {
        "Mbox Embedded"
    }
}

#[cfg(test)]
mod tests {
    use std::path;

    use tokio::sync::mpsc::Receiver;
    use tokio::task::JoinHandle;

    use test_utils::temp_path;

    use crate::processing::ProcessContextBuilder;

    use super::*;

    type ProcessFuture = JoinHandle<anyhow::Result<()>>;
    type OutputReceiver = Receiver<Result<ProcessOutput, anyhow::Error>>;

    fn processor_with_context() -> (MboxEmbeddedProcessor, ProcessContext, Receiver<Result<ProcessOutput, anyhow::Error>>) {
        let (output_sink, outputs) = tokio::sync::mpsc::channel(10);
        let ctx = ProcessContextBuilder::new("application/mbox", vec![], output_sink).build();
        (MboxEmbeddedProcessor, ctx, outputs)
    }

    fn process(path: path::PathBuf) -> Result<(ProcessFuture, OutputReceiver), anyhow::Error> {
        let (processor, ctx, output_rx) = processor_with_context();
        let proc_fut = tokio::spawn(async move {
            processor.process(ctx, &path, temp_path()?, "checksum").await
        });
        Ok((proc_fut, output_rx))
    }

    #[tokio::test]
    async fn test_process() -> anyhow::Result<()> {
        let path = path::PathBuf::from("../resources/mbox/ubuntu-no-small.mbox");
        let (proc_fut, mut output_rx) = process(path)?;

        let mut outputs = vec![];
        while let Some(output) = output_rx.recv().await {
            match output? {
                ProcessOutput::Processed(_, _) => panic!("Expected embedded metadata.json"),
                ProcessOutput::Embedded(state, data, _) => outputs.push((state, data))
            }
        }
        proc_fut.await??;

        // Sort to make the test deterministic
        outputs.sort_by(|o0, o1| o0.1.checksum.cmp(&o1.1.checksum));

        assert_eq!(outputs.len(), 2);

        let (state, ctx) = &outputs[0];
        assert_eq!(ctx.mimetype, "message/rfc822");
        assert_eq!(ctx.checksum, "88dde30cbe134ce0dd8aa0979546646a");
        assert!(state.id_chain.is_empty());

        let (state, ctx) = &outputs[1];
        assert_eq!(ctx.mimetype, "message/rfc822");
        assert_eq!(ctx.checksum, "c694e99230b3cbf36d8aef4131596864");
        assert!(state.id_chain.is_empty());

        Ok(())
    }

    #[tokio::test]
    async fn test_process_large_file() -> anyhow::Result<()> {
        let path = path::PathBuf::from("../resources/mbox/ubuntu-no.mbox");
        let (proc_fut, mut output_rx) = process(path)?;

        let mut output_count = 0;
        while let Some(output) = output_rx.recv().await {
            match output? {
                ProcessOutput::Processed(_, _) => panic!("Expected embedded metadata.json"),
                ProcessOutput::Embedded(_, data, _) => {
                    output_count += 1;
                    assert_eq!(data.mimetype, "message/rfc822");
                }
            }
        }
        proc_fut.await??;

        assert_eq!(output_count, 344);
        Ok(())
    }
}

