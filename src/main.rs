#![allow(rustdoc::bare_urls)]

use anyhow::Result;
use clap::Parser;

mod ao3;
mod gelbooru;
mod jnovel;
mod mangadex;
mod qq;
mod royalroad;
mod seasonal_anime;
mod tfgames;
mod vn_news;

#[derive(Debug, Parser)]
#[clap(
    name = "rss-scrapers",
    about = "Tool for scraping various sites and constructing rss feeds"
)]
pub struct Opt {
    #[command(subcommand)]
    cmd: Command,

    #[arg(long, global = true)]
    etag: Option<String>,
}

#[derive(Debug, Parser)]
enum Command {
    /// Archive Of Our Own
    Ao3 {
        /// The story id from the URL.
        /// https://archiveofourown.org/works/1234 has a story id of 1234
        #[arg(allow_hyphen_values = true)]
        story_id: String,
    },
    /// Gelbooru Rss
    /// Uses $HOME/.rss/geltagblacklist
    Gelbooru {
        #[arg(allow_hyphen_values = true, required=true, num_args=1..)]
        query: Vec<String>,
    },
    /// Jnovel-club series
    Jnovel {
        /// The jnovel title slug, from after /series/ in the title.
        /// https://j-novel.club/series/ab-cd has a title slug of ab-cd
        #[arg(allow_hyphen_values = true)]
        title_slug: String,
    },
    /// Mangadex series
    Mangadex {
        /// Mangadex series UUID
        /// https://mangadex.org/title/975f3334-8395-4393-84a2-50fcaccbcdc0 has a UUID of
        /// 975f3334-8395-4393-84a2-50fcaccbcdc0
        #[arg(allow_hyphen_values = true)]
        series: String,
    },
    // QQ
    QQ {
        /// Thread ID
        /// /threads/ab-cd.1234 has an ID of ab-cd.1234
        #[arg(allow_hyphen_values = true)]
        thread_id: String,
    },
    RoyalRoad,
    SeasonalAnime,
    Tfgames {
        /// Game ID
        /// https://tfgames.site/?module=viewgame&id=1234 has an ID of 1234
        #[arg(allow_hyphen_values = true)]
        game_id: String,
    },
    VnNews,
}


fn main() -> Result<()> {
    let opt = Opt::parse();

    match opt.cmd {
        Command::Ao3 { story_id } => ao3::get(story_id),
        Command::Gelbooru { query } => gelbooru::get(query),
        Command::Jnovel { title_slug } => jnovel::get(title_slug),
        Command::Mangadex { series } => mangadex::get(series),
        Command::QQ { thread_id } => qq::get(thread_id, opt.etag),
        Command::RoyalRoad => royalroad::get(),
        Command::SeasonalAnime => seasonal_anime::get(),
        Command::Tfgames { game_id } => tfgames::get(game_id),
        Command::VnNews => vn_news::get(),
    }
}
