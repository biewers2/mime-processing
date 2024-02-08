use std::process::ExitStatus;

use lazy_static::lazy_static;
use tokio::io::{AsyncRead, AsyncWrite};
use crate::{stream_command, trim_to_string};

const PROGRAM: &str = "wkhtmltopdf";

const DEFAULT_ARGS: [&str; 15] = [
    "--quiet",
    "--encoding",
    "utf-8",
    "--disable-external-links",
    "--disable-internal-links",
    "--disable-forms",
    "--disable-local-file-access",
    "--disable-javascript",
    "--disable-toc-back-links",
    "--disable-plugins",
    "--proxy",
    "bogusproxy",
    "--proxy-hostname-lookup",
    "-",
    "-",
];

/// The type of the singleton instance of the `HtmlToPdf` service.
///
pub type HtmlToPdfService = Box<HtmlToPdf>;

lazy_static! {
    static ref HTML_TO_PDF: HtmlToPdfService = Box::<HtmlToPdf>::default();
}

/// Returns the singleton instance of the `HtmlToPdf` service.
///
pub fn html_to_pdf() -> &'static HtmlToPdfService {
    &HTML_TO_PDF
}

/// The output of the `HtmlToPdf` service.
///
pub struct HtmlToPdfOutput {
    /// The exit status of the call to the `HtmlToPdf` CLI tool.
    ///
    pub exit_status: ExitStatus,

    /// The stderr of the call to the `HtmlToPdf` CLI tool.
    ///
    pub error: String,
}

/// The `HtmlToPdf` service.
///
#[derive(Default)]
pub struct HtmlToPdf;

impl HtmlToPdf {
    /// Run the `HtmlToPdf` service.
    ///
    /// # Arguments
    ///
    /// * `input` - An asynchronous reader representing HTML content to read into stdin of the `HtmlToPdf` CLI tool.
    /// * `output` - An asynchronous writer representing PDF content to write from stdout of the `HtmlToPdf` CLI tool.
    ///
    /// # Returns
    ///
    /// * `Ok(HtmlToPdfOutput)` - If the `HtmlToPdf` CLI tool was run successfully.
    /// * `Err(_)` - If there was an error running the `PdfToImage` CLI tool.
    ///
    pub async fn run<R, W>(&self, mut input: R, mut output: W) -> Result<HtmlToPdfOutput, anyhow::Error>
    where
        R: AsyncRead + Unpin,
        W: AsyncWrite + Unpin,
    {
        let mut error = vec![];
        let exit_value = stream_command(
            PROGRAM,
            &DEFAULT_ARGS,
            Some(&mut input),
            Some(&mut output),
            Some(&mut error),
        ).await?;

        Ok(HtmlToPdfOutput {
            exit_status: exit_value,
            error: trim_to_string(&error),
        })
    }
}

#[cfg(test)]
mod tests {
    use std::any::{Any, TypeId};
    

    
    use crate::test_utils::assert_command_successful;

    use super::*;

    #[tokio::test]
    async fn check_wkhtmltopdf_installed() {
        assert_command_successful("which wkhtmltopdf").await.unwrap();
    }

    #[test]
    fn check_singleton() {
        assert_eq!(html_to_pdf().type_id(), TypeId::of::<Box<HtmlToPdf>>());
    }

    #[tokio::test]
    async fn test_html_to_pdf() {
        let input = b"hello world".to_vec();
        let mut pdf = vec![];

        let output = html_to_pdf().run(input.as_ref(), &mut pdf).await.unwrap();

        assert!(output.exit_status.success());
        assert_eq!(output.error, "");
        assert_ne!(pdf.len(), 0);
    }
}
