use std::collections::BTreeMap;
use std::fs::File;
use std::io::{BufReader, Cursor, ErrorKind};
use std::path::PathBuf;
use std::sync::Arc;
use std::thread;
use std::time::Duration;

use anyhow::{Context, Result};
use reqwest::blocking::multipart::Form;
use reqwest::header::{ETAG, IF_NONE_MATCH};
use reqwest::StatusCode;
use reqwest_cookie_store::{CookieStore, CookieStoreMutex};
use rss::extension::{Extension, ExtensionMap};
use rss::Channel;
use scraper::{Html, Selector};
use serde::Deserialize;


#[derive(Debug, Deserialize)]
struct Config {
    username: String,
    password: String,
    cookie_jar: PathBuf,
}

pub fn get(thread_id: String, last_etag: Option<String>) -> Result<()> {
    let config: Config = awconf::load_config("qq-rss", None::<&str>, None::<&str>)?.0;

    let cookie_store = match File::open(&config.cookie_jar) {
        Ok(f) => CookieStore::load_json(BufReader::new(f)).unwrap(),
        Err(e) if e.kind() == ErrorKind::NotFound => CookieStore::new(None),
        Err(e) => return Err(e.into()),
    };

    let cookie_store = Arc::new(CookieStoreMutex::new(cookie_store));

    let client = reqwest::blocking::Client::builder()
        .cookie_provider(cookie_store.clone())
        .build()?;

    let url = format!(
        "https://forum.questionablequesting.com/threads/{thread_id}/threadmarks.rss?category_id=1"
    );
    let mut req = client.get(&url);
    // Only bother setting etag the first time, just in case weird things happen
    if let Some(etag) = last_etag {
        req = req.header(IF_NONE_MATCH, etag);
    }
    let resp = req.send()?;

    if resp.status() == StatusCode::NOT_MODIFIED {
        println!("not modified");
        return Ok(());
    }

    let mut etag = resp
        .headers()
        .get(ETAG)
        .and_then(|etag| etag.to_str().ok())
        .map(ToString::to_string);

    let text = resp.text()?;
    let feed = Channel::read_from(Cursor::new(&text));

    let mut feed = match feed {
        Ok(feed) => feed,
        Err(_e) => {
            let text = client.get("https://forum.questionablequesting.com").send()?.text()?;
            let doc = Html::parse_document(&text);

            // We don't need a specific xsrf token for the login page, any valid token will do.
            let xf_token = doc
                .select(&Selector::parse("input[name=\"_xfToken\"]").unwrap())
                .next()
                .context("No xfToken in initial response")?
                .attr("value")
                .context("xfToken had no value")?
                .to_string();

            thread::sleep(Duration::from_secs(1));

            let form = Form::new()
                .text("login", config.username)
                .text("password", config.password)
                .text("remember", "1")
                .text("_xfToken", xf_token);

            client
                .post("https://forum.questionablequesting.com/login/login")
                .multipart(form)
                .send()?
                .text()?;

            thread::sleep(Duration::from_secs(1));

            let resp = client.get(&url).send()?;
            etag = resp
                .headers()
                .get(ETAG)
                .and_then(|etag| etag.to_str().ok())
                .map(ToString::to_string);
            Channel::read_from(Cursor::new(resp.text()?))?
        }
    };

    {
        let mut f = File::create(&config.cookie_jar)?;
        let store = cookie_store.lock().unwrap();
        store.save_json(&mut f).unwrap();
    }

    // Fix the link to the thread
    feed.set_link(format!("https://forum.questionablequesting.com/threads/{thread_id}"));
    if let Some(etag) = etag {
        let mut extensions = ExtensionMap::new();
        let mut ext = Extension::default();
        ext.set_name("aw-rss:etag".to_string());
        ext.set_value(Some(etag));
        let mut map = BTreeMap::new();
        map.insert(String::new(), vec![ext]);
        extensions.insert(String::new(), map);

        feed.set_extensions(extensions);
    }

    println!("{}", feed.to_string());

    Ok(())
}
