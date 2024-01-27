use anyhow::Result;
use clap::Parser;

mod ao3;
mod jnovel;
mod mangadex;
mod seasonal_anime;

#[derive(Debug, Parser)]
#[clap(
    name = "rss-scrapers",
    about = "Tool for scraping various sites and constructing rss feeds"
)]
pub struct Opt {
    // #[arg(short, long, value_parser)]
    /// Override the selected config.
    // awconf: Option<PathBuf>,

    #[command(subcommand)]
    cmd: Command,
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
    SeasonalAnime,
}


// #[derive(Debug, Deserialize)]
// pub struct Credentials {
//     #[serde(default)]
//     xx_username: String,
//     #[serde(default)]
//     xx_password: String,
// }

fn main() -> Result<()> {
    let opt = Opt::parse();

    match opt.cmd {
        Command::Ao3 { story_id } => ao3::get(story_id),
        Command::Jnovel { title_slug } => jnovel::get(title_slug),
        Command::Mangadex { series } => mangadex::get(series),
        Command::SeasonalAnime => seasonal_anime::get(),
    }
}
