use std::io::Write;
use anyhow::Context;

use mail_parser::Message;

use services::{CommandError, html_to_pdf};

use crate::pdf::rfc822::html_message_visitor::HtmlMessageVisitor;
use crate::pdf::rfc822::transformer::MessageTransformer;
use crate::pdf::Rfc822PdfProcessor;

impl Rfc822PdfProcessor {
    pub async fn render_pdf(&self, message: &Message<'_>, writer: &mut impl Write) -> Result<(), anyhow::Error> {
        let transformer = MessageTransformer::new(Box::<HtmlMessageVisitor>::default());

        let mut html = Vec::<u8>::new();
        let mut pdf = Vec::new();

        transformer.transform(message, &mut html)
            .context("failed to transform message")?;

        self.render_html_to_pdf(html.to_vec(), &mut pdf).await?;
        writer.write_all(pdf.as_ref())
            .context("failed to write pdf to file")?;

        Ok(())
    }

    async fn render_html_to_pdf(&self, html: Vec<u8>, output: &mut Vec<u8>) -> Result<(), anyhow::Error> {
        let result = html_to_pdf().run(html.as_ref(), output).await;

        if let Err(e) = &result {
            if let Some(e) = e.downcast_ref::<CommandError>() {
                if e.exit_code().is_some_and(|code| code == 1) {
                    return Ok(())
                }
            }
        }

        result.context("failed to render html to pdf")?;
        Ok(())
    }
}
