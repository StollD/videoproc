use std::{path::Path, process::Command};

use execute::Execute;
use json::JsonValue;
use tempdir::TempDir;

use crate::{logging, mkv, utils::StrVec};

pub fn run(stream: &mkv::Stream, output: &Path, options: &JsonValue) -> Result<mkv::Stream, ()> {
	if !options.has_key("codec") {
		logging::error!("Did not find codec!");
		return Err(());
	}

	let codec = options["codec"].to_string();
	let path = output.join(&stream.id).with_extension("enc.mkv");

	logging::info!("Encoding stream with codec {}", codec);

	let mut args = Vec::<String>::new();

	args.push_str("-i");
	args.push(stream.path.to_str().unwrap().to_string());

	let map = format!("0:{}", stream.index);
	args.push_str("-map");
	args.push(map);

	if stream.aspect.is_some() {
		args.push_str("-aspect");
		args.push(stream.aspect.as_ref().unwrap().clone());
	}

	if codec == "ac3" {
		let dsurmode = stream.dsurmode.map(|d| d.to_string());
		if let Some(dsurmode) = dsurmode {
			args.push_str("-dsur_mode");
			args.push(dsurmode);
		}
	}

	for entry in options.entries() {
		let mut key = entry.0;
		let mut val = entry.1.to_string();

		if key == "bitrate" {
			if stream.streamtype == "video" {
				key = "b:v"
			} else if stream.streamtype == "audio" {
				key = "b:a";
				val = format!("{}*{}", val, stream.channels.unwrap());
			}
		}

		args.push(format!("-{}", key));
		args.push(val);
	}

	// Is this a two-pass encode?
	if stream.streamtype == "video" && options.has_key("bitrate") {
		let temp = TempDir::new("videoproc");
		if let Err(err) = temp {
			logging::error!("Failed to create directory: {}", err);
			return Err(());
		}

		let temp = temp.unwrap();
		let log = temp.path().join("ffmpeg2pass");

		let mut p1 = args.clone();

		p1.push_str("-pass");
		p1.push_str("1");

		p1.push_str("-passlogfile");
		p1.push_str(log.to_str().unwrap());

		p1.push_str("-f");
		p1.push_str("null");
		p1.push_str("-");

		let cmd = Command::new("ffmpeg")
			.args(p1)
			.execute_check_exit_status_code(0);

		if let Err(err) = cmd {
			logging::error!("Failed to run ffmpeg: {}", err);
			return Err(());
		}

		let mut p2 = args.clone();

		p2.push_str("-pass");
		p2.push_str("2");

		p2.push_str("-passlogfile");
		p2.push_str(log.to_str().unwrap());

		p2.push_str("-y");
		p2.push_str(path.to_str().unwrap());

		let cmd = Command::new("ffmpeg")
			.args(p2)
			.execute_check_exit_status_code(0);

		if let Err(err) = cmd {
			logging::error!("Failed to run ffmpeg: {}", err);
			return Err(());
		}
	} else {
		args.push_str("-y");
		args.push_str(path.to_str().unwrap());

		let cmd = Command::new("ffmpeg")
			.args(args)
			.execute_check_exit_status_code(0);

		if let Err(err) = cmd {
			logging::error!("Failed to run ffmpeg: {}", err);
			return Err(());
		}
	}

	let mut new = stream.clone();
	new.path = path;
	new.index = 0;

	if codec != "copy" {
		new.codec = Some(codec);
	}

	Ok(new)
}