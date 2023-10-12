use std::{path::Path, process::Command};

use execute::Execute;

use crate::{logging, mkv, utils};

pub fn run(stream: &mkv::Stream, output: &Path) -> Result<mkv::Stream, ()> {
	logging::info!("Extracting stream");

	if stream.streamtype != "chapters" {
		extract_stream(stream, output)
	} else {
		extract_chapters(stream, output)
	}
}

fn extract_stream(stream: &mkv::Stream, output: &Path) -> Result<mkv::Stream, ()> {
	let path = output.join(&stream.id).with_extension("mkv");

	let raw = path.with_extension("temp.raw");

	let cmd = Command::new("mkvextract")
		.arg(&stream.path)
		.arg("tracks")
		.arg(format!("{}:{}", stream.index, raw.to_str().unwrap()))
		.execute_check_exit_status_code(0);

	if let Err(err) = cmd {
		logging::error!("Failed to extract stream: {}", err);
		return Err(());
	}

	if stream.codec.as_deref() == Some("dvd_subtitle") {
		let idx = raw.with_extension("idx");
		let sub = raw.with_extension("sub");

		let cmd = Command::new("mkvmerge")
			.arg("-o")
			.arg(&path)
			.arg(&idx)
			.arg(&sub)
			.execute();

		if let Err(err) = cmd {
			logging::error!("Failed to create MKV: {}", err);
			return Err(());
		}

		let cmd = cmd.unwrap();

		match cmd {
			Some(0) => {}
			Some(1) => {}
			_ => {
				logging::error!("Failed to create MKV: unexpected exit code");
				return Err(());
			}
		};

		let err = std::fs::remove_file(&idx);
		if let Err(err) = err {
			logging::error!("Failed to remove file: {}", err);
			return Err(());
		}

		let err = std::fs::remove_file(&sub);
		if let Err(err) = err {
			logging::error!("Failed to remove file: {}", err);
			return Err(());
		}
	} else {
		let cmd = Command::new("mkvmerge")
			.arg("-o")
			.arg(&path)
			.arg(&raw)
			.execute();

		if let Err(err) = cmd {
			logging::error!("Failed to create MKV: {}", err);
			return Err(());
		}

		let cmd = cmd.unwrap();

		match cmd {
			Some(0) => {}
			Some(1) => {}
			_ => {
				logging::error!("Failed to create MKV: unexpected exit code");
				return Err(());
			}
		};

		let err = std::fs::remove_file(&raw);
		if let Err(err) = err {
			logging::error!("Failed to remove file: {}", err);
			return Err(());
		}
	}

	let mut new = stream.clone();
	new.path = path;
	new.index = 0;

	Ok(new)
}

fn extract_chapters(stream: &mkv::Stream, output: &Path) -> Result<mkv::Stream, ()> {
	let path = output.join(&stream.id).with_extension("txt");

	let cmd = Command::new("ffmpeg")
		.arg("-i")
		.arg(&stream.path)
		.arg("-f")
		.arg("ffmetadata")
		.arg("-y")
		.arg("-")
		.output();

	let cmd = utils::check_output(cmd);
	if let Err(err) = cmd {
		logging::error!("Failed to extract chapters: {}", err);
		return Err(());
	}

	let chapters = String::from_utf8(cmd.unwrap().stdout);
	if let Err(err) = chapters {
		logging::error!("Failed to decode ffmpeg output: {}", err);
		return Err(());
	}

	let chapters = chapters.unwrap();
	let mut new = String::new();

	// Remove chapter names
	// For MakeMKV these are just generic "Chapter N" values
	for line in chapters.split('\n') {
		if line.starts_with("title") {
			continue;
		}

		new.push_str(line);
		new.push('\n');
	}

	let err = std::fs::write(&path, new);
	if let Err(err) = err {
		logging::error!("Failed to write chapters: {}", err);
		return Err(());
	}

	let mut new = stream.clone();
	new.path = path;

	Ok(new)
}
