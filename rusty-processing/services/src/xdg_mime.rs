use anyhow::Context;
use std::io::Cursor;
use std::path::Path;

use lazy_static::lazy_static;

use crate::{stream_command, trim_to_string};

/// The type of the singleton instance of the `XdgMime` service.
///
pub type XdgMimeService = Box<XdgMime>;

lazy_static! {
    static ref XDG_MIME: XdgMimeService = Box::<XdgMime>::default();
}

/// Returns the singleton instance of the `xdg-mime` service.
pub fn xdg_mime() -> &'static XdgMimeService {
    &XDG_MIME
}

/// The `xdg-mime` service.
///
#[derive(Default)]
pub struct XdgMime;

impl XdgMime {
    /// Run the `xdg-mime` service to identify the mimetype of a file.
    ///
    /// # Arguments
    ///
    /// * `path` - The path to the file to identify.
    ///
    /// # Returns
    ///
    /// The mimetype of the file.
    ///
    pub async fn query_filetype(&self, path: impl AsRef<Path>) -> Result<String, anyhow::Error> {
        let path_str = path.as_ref().to_string_lossy().to_string();

        let mut output = vec![];
        let mut error = vec![];
        stream_command(
            "xdg-mime",
            &["query", "filetype", &path_str],
            Option::<Cursor<Vec<u8>>>::None,
            Some(&mut output),
            Some(&mut error),
        )
        .await
        .map_err(|error| anyhow::anyhow!("{}", error))
        .context(format!(
            "'xdg-mime' failed to detect mimetype: {}",
            trim_to_string(&error)
        ))?;

        Ok(trim_to_string(&output))
    }
}

#[cfg(test)]
mod tests {
    use std::any::{Any, TypeId};

    use super::*;

    #[test]
    fn check_singleton() {
        assert_eq!(xdg_mime().type_id(), TypeId::of::<Box<XdgMime>>());
    }

    #[tokio::test]
    async fn test_query_filetype() {
        let cases = vec![
            ("../resources/mbox/ubuntu-no-small.mbox", "application/mbox"),
            ("../resources/rfc822/headers-small.eml", "message/rfc822"),
            ("../resources/jpg/PA280041.JPG", "image/jpeg"),
        ];

        for (path, expected) in cases {
            let result = xdg_mime().query_filetype(path).await;

            assert!(result.is_ok(), "failed to query filetype for '{}'", path);
            assert_eq!(result.unwrap(), expected);
        }
    }

    #[tokio::test]
    async fn test_query_filetype_missing_path() {
        let expected_err = "\
'xdg-mime' failed to detect mimetype: \
xdg-mime: file 'path-does-not-exist' does not exist";
        let path = "path-does-not-exist";

        let result = xdg_mime().query_filetype(path).await;

        assert!(result.is_err());
        assert_eq!(result.unwrap_err().to_string(), expected_err);
    }
}
