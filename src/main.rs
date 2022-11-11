use std::{path::PathBuf, process::Command};

use argparse::{ArgumentParser, Store};
use execute::Execute;

mod config;
mod filters;
mod input;
mod logging;
mod mkv;
mod select;
mod utils;

fn main() {
	std::process::exit(run());
}

fn run() -> i32 {
	let _guard = logging::init();

	let mut config = PathBuf::from("config");
	let mut input = PathBuf::from("input");
	let mut working = PathBuf::from("working");
	let mut output = PathBuf::from("output");

	{
		let mut parser = ArgumentParser::new();
		parser
			.refer(&mut config)
			.add_option(&["--config"], Store, "Config directory");
		parser
			.refer(&mut input)
			.add_option(&["--input"], Store, "Input directory");
		parser
			.refer(&mut working)
			.add_option(&["--working"], Store, "Working directory");
		parser
			.refer(&mut output)
			.add_option(&["--output"], Store, "Output directory");
		parser.parse_args_or_exit();
	}

	/*
	 * Check if directories exist
	 */

	if !config.exists() || !config.is_dir() {
		logging::error!("{} is not a directory!", config.to_str().unwrap());
		return 1;
	}

	if !input.exists() || !input.is_dir() {
		logging::error!("{} is not a directory!", input.to_str().unwrap());
		return 1;
	}

	if working.exists() && !working.is_dir() {
		logging::error!("{} is not a directory!", working.to_str().unwrap());
		return 1;
	}

	if output.exists() && !output.is_dir() {
		logging::error!("{} is not a directory!", output.to_str().unwrap());
		return 1;
	}

	/*
	 * Create working and output directories
	 */

	let err = std::fs::create_dir_all(&working);
	if let Err(err) = err {
		logging::error!("Failed to create working directory: {}", err);
		return 1;
	}

	let err = std::fs::create_dir_all(&output);
	if let Err(err) = err {
		logging::error!("Failed to create output directory: {}", err);
		return 1;
	}

	config = config.canonicalize().unwrap();
	input = input.canonicalize().unwrap();
	working = working.canonicalize().unwrap();
	output = output.canonicalize().unwrap();

	/*
	 * Test commandline programs
	 */

	let cmd = Command::new("ffmpeg")
		.arg("-version")
		.execute_check_exit_status_code(0);

	if let Err(err) = cmd {
		logging::error!("Failed to run ffmpeg: {}", err);
		return 1;
	}

	let cmd = Command::new("ffprobe")
		.arg("-version")
		.execute_check_exit_status_code(0);

	if let Err(err) = cmd {
		logging::error!("Failed to run ffprobe: {}", err);
		return 1;
	}

	let cmd = Command::new("vspipe")
		.arg("--version")
		.execute_check_exit_status_code(0);

	if let Err(err) = cmd {
		logging::error!("Failed to run vspipe: {}", err);
		return 1;
	}

	let cmd = Command::new("mediainfo")
		.arg("--version")
		.execute_check_exit_status_code(0);

	if let Err(err) = cmd {
		logging::error!("Failed to run mediainfo: {}", err);
		return 1;
	}

	let cmd = Command::new("mkvmerge")
		.arg("--version")
		.execute_check_exit_status_code(0);

	if let Err(err) = cmd {
		logging::error!("Failed to run mkvmerge: {}", err);
		return 1;
	}

	let cmd = Command::new("mkvextract")
		.arg("--version")
		.execute_check_exit_status_code(0);

	if let Err(err) = cmd {
		logging::error!("Failed to run mkvextract: {}", err);
		return 1;
	}

	let cmd = Command::new("d2vwitch")
		.arg("--version")
		.execute_check_exit_status_code(0);

	if let Err(err) = cmd {
		logging::error!("Failed to run d2vwitch: {}", err);
		return 1;
	}

	let cmd = Command::new("avs2pipemod64").execute_check_exit_status_code(255);
	if let Err(err) = cmd {
		logging::error!("Failed to run avs2pipemod64: {}", err);
		return 1;
	}

	// Process inputs
	let _ = input::process(&config, &input, &working, &output);

	0
}
