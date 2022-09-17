use std::env;

use anyhow::{Context, Result};
use scraper::{Html, Selector};
use serde_json::Value;
use sha2::{Digest, Sha256};

fn main() -> Result<()> {
    let mut args = env::args().skip(1);
    let id = args.next().context("id argument missing")?;
    let version = args.next();

    let (publisher, name) = id.split_once('.').context("invalid extension id")?;
    let page = ureq::get(&format!(
        "https://marketplace.visualstudio.com/items?itemName={publisher}.{name}"
    ))
    .call()?;

    let html = Html::parse_document(&page.into_string()?);
    let json = html
        .select(&Selector::parse(".rhs-content .jiContent").unwrap())
        .next()
        .context("display name node not found")?
        .text()
        .next()
        .context("display name text not found")?;

    let data: Value = dbg!(serde_json::from_str(json)?);
    let latest = data
        .get("Versions")
        .context("version list not found")?
        .get(0)
        .context("version item not found")?
        .get("version")
        .context("version number not found")?
        .as_str()
        .context("version is not a string")?;

    let vsix = ureq::get(
        &format!("https://marketplace.visualstudio.com/_apis/public/gallery/publishers/{publisher}/vsextensions/{name}/{latest}/vspackage")
    ).call()?.into_string()?;
    let vsix_bytes = vsix.as_bytes();

    let mut hasher = Sha256::new();
    hasher.update(vsix_bytes);

    let hash = hasher.finalize();
    let sha256 = base64::encode(hash);

    let current = version.as_deref().unwrap_or(latest);
    if current != latest {
        println!("{{ publisher = \"{publisher}\", name = \"{name}\", version = \"{latest}\", sha256 = \"{sha256}\" }}")
    }

    Ok(())
}
