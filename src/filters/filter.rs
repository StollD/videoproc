use std::path::{Path, PathBuf};

use json::JsonValue;

use crate::{logging, mkv, utils};

use super::{avisynth, dolby, encode, extract, offset, pitch, speed, tempo, vapoursynth};

pub fn run(cfg: &JsonValue, stream: &mkv::Stream, working: &Path) -> Result<mkv::Stream, ()> {
	let filters = &cfg["filters"];

	// Create working directory
	let dir = working.join(&stream.id);

	let err = std::fs::create_dir_all(&dir);
	if let Err(err) = err {
		let name = dir.to_str().unwrap();

		logging::error!("Failed to create directory {}: {}", name, err);
		return Err(());
	}

	// Extract the stream
	let mut stage = 1;
	let mut current = run_stage(stream, &dir, stage, extract::run)?;

	// Normalize Dolby audio, unless the stream is copied
	for filter in filters.members() {
		let name = &filter["$type"];

		if name == "encode" {
			let newcodec = &filter["codec"];

			if current.codec.as_deref() == Some("ac3") && newcodec != "copy" {
				stage += 1;
				current = run_stage(&current, &dir, stage, dolby::normalize)?;
			}
		}
	}

	// Run other filters
	for filter in filters.members() {
		let name = &filter["$type"];
		let path = String::from(working.to_str().unwrap());

		if name == "offset" {
			let mut secs = Ok(0.0);

			for entry in filter.entries() {
				let key = entry.0;
				let value = entry.1;

				let regex = fnmatch_regex::glob_to_regex(format!("*{key}*").as_str());
				if regex.is_err() {
					continue;
				}

				let regex = regex.unwrap();
				if regex.is_match(&path) {
					secs = value.as_str().unwrap_or("0.0").parse::<f32>();
					break;
				}
			}

			stage += 1;
			current = run_stage(&current, &dir, stage, |s, o| {
				offset::change(s, o, secs.unwrap())
			})?;
		}

		if name == "encode" && stream.streamtype != "chapters" {
			let mut options = filter.clone();
			options.remove("$type");

			stage += 1;
			current = run_stage(&current, &dir, stage, |s, o| encode::run(s, o, &options))?;
		}

		if current.streamtype == "video" {
			if name == "vapoursynth" {
				let filter = PathBuf::from(filter["filter"].to_string());

				stage += 1;
				current = run_stage(&current, &dir, stage, |s, o| {
					vapoursynth::run(s, o, &filter)
				})?;
			}

			if name == "avisynth" {
				let filter = PathBuf::from(filter["filter"].to_string());

				stage += 1;
				current = run_stage(&current, &dir, stage, |s, o| avisynth::run(s, o, &filter))?;
			}

			if name == "speed" {
				let framerate = &filter["framerate"];
				if framerate.is_null() {
					logging::error!("Missing framerate!");
					return Err(());
				}

				let framerate = utils::framerate(framerate.as_str().unwrap());

				stage += 1;
				current = run_stage(&current, &dir, stage, |s, o| {
					speed::change_video(s, o, framerate)
				})?;
			}
		}

		if current.streamtype == "audio" && name == "speed" {
			let infps = &filter["input"];
			if infps.is_null() {
				logging::error!("Missing input framerate!");
				return Err(());
			}

			let outfps = &filter["output"];
			if outfps.is_null() {
				logging::error!("Missing output framerate!");
				return Err(());
			}

			let infps = utils::framerate(infps.as_str().unwrap());
			let outfps = utils::framerate(outfps.as_str().unwrap());

			stage += 1;
			current = run_stage(&current, &dir, stage, |s, o| {
				speed::change_audio(s, o, infps, outfps)
			})?;
		}

		if current.streamtype == "audio" && name == "tempo" {
			let infps = &filter["input"];
			if infps.is_null() {
				logging::error!("Missing input framerate!");
				return Err(());
			}

			let outfps = &filter["output"];
			if outfps.is_null() {
				logging::error!("Missing output framerate!");
				return Err(());
			}

			let infps = utils::framerate(infps.as_str().unwrap());
			let outfps = utils::framerate(outfps.as_str().unwrap());

			stage += 1;
			current = run_stage(&current, &dir, stage, |s, o| {
				tempo::change(s, o, infps, outfps)
			})?;
		}

		if current.streamtype == "audio" && name == "pitch" {
			let infps = &filter["input"];
			if infps.is_null() {
				logging::error!("Missing input framerate!");
				return Err(());
			}

			let outfps = &filter["output"];
			if outfps.is_null() {
				logging::error!("Missing output framerate!");
				return Err(());
			}

			let infps = utils::framerate(infps.as_str().unwrap());
			let outfps = utils::framerate(outfps.as_str().unwrap());

			stage += 1;
			current = run_stage(&current, &dir, stage, |s, o| {
				pitch::change(s, o, infps, outfps)
			})?;
		}

		if current.streamtype == "subtitle" && name == "speed" {
			let infps = &filter["input"];
			if infps.is_null() {
				logging::error!("Missing input framerate!");
				return Err(());
			}

			let outfps = &filter["output"];
			if outfps.is_null() {
				logging::error!("Missing output framerate!");
				return Err(());
			}

			let infps = utils::framerate(infps.as_str().unwrap());
			let outfps = utils::framerate(outfps.as_str().unwrap());

			stage += 1;
			current = run_stage(&current, &dir, stage, |s, o| {
				speed::change_subtitles(s, o, infps, outfps)
			})?;
		}

		if current.streamtype == "chapters" && name == "speed" {
			let infps = &filter["input"];
			if infps.is_null() {
				logging::error!("Missing input framerate!");
				return Err(());
			}

			let outfps = &filter["output"];
			if outfps.is_null() {
				logging::error!("Missing output framerate!");
				return Err(());
			}

			let infps = utils::framerate(infps.as_str().unwrap());
			let outfps = utils::framerate(outfps.as_str().unwrap());

			stage += 1;
			current = run_stage(&current, &dir, stage, |s, o| {
				speed::change_chapters(s, o, infps, outfps)
			})?;
		}
	}

	Ok(current)
}

fn run_stage<F>(
	stream: &mkv::Stream,
	working: &Path,
	stage: u32,
	action: F,
) -> Result<mkv::Stream, ()>
where
	F: FnOnce(&mkv::Stream, &Path) -> Result<mkv::Stream, ()>,
{
	let dir = working.join(format!("Stage{stage}"));

	let err = std::fs::create_dir_all(&dir);
	if let Err(err) = err {
		let name = dir.to_str().unwrap();

		logging::error!("Failed to create directory {}: {}", name, err);
		return Err(());
	}

	// If this stage was already committed, don't run it again
	let commit = dir.join("commit").with_extension("json");
	if commit.exists() {
		return mkv::Stream::load(&commit);
	}

	let new = action(stream, &dir)?;

	// Commit the new stream
	let err = new.save(&commit);
	if let Err(err) = err {
		logging::error!("Failed to commit stage {}: {}", stage, err);
		return Err(());
	}

	// Make sure we dont clean up stage 0
	let prev = stage - 1;
	if prev == 0 {
		return Ok(new);
	}

	// Clean up the old stream
	let err = stream.cleanup();
	if let Err(err) = err {
		logging::error!("Failed to cleanup stage {}: {}", prev, err);
		return Err(());
	}

	Ok(new)
}
