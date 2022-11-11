use std::{path::Path, process::Command};

use execute::Execute;

use crate::{logging, mkv, utils};

pub fn change_video(
	stream: &mkv::Stream,
	output: &Path,
	framerate: (u32, u32),
) -> Result<mkv::Stream, ()> {
	let speedup = utils::speedup(stream.framerate.unwrap(), framerate);
	let path = output.join(&stream.id).with_extension("speed.mkv");

	logging::info!("Changing speed by {}", speedup);

	let mut args = Vec::<&str>::new();

	args.push("-o");
	args.push(path.to_str().unwrap());

	let duration = format!("{}:{}/{}p", stream.index, framerate.0, framerate.1);
	args.push("--default-duration");
	args.push(&duration);

	let timing = format!("{}:true", stream.index);
	args.push("--fix-bitstream-timing-information");
	args.push(&timing);

	args.push(stream.path.to_str().unwrap());

	let cmd = Command::new("mkvmerge")
		.args(args)
		.execute_check_exit_status_code(0);

	if let Err(err) = cmd {
		logging::error!("Failed to run mkvmerge: {}", err);
		return Err(());
	}

	let mut new = stream.clone();
	new.path = path.clone();
	new.framerate = Some(framerate);
	new.offset /= speedup;
	new.duration /= speedup;

	Ok(new)
}

pub fn change_audio(
	stream: &mkv::Stream,
	output: &Path,
	infps: (u32, u32),
	recfps: (u32, u32),
	outfps: (u32, u32),
) -> Result<mkv::Stream, ()> {
	let speedup = utils::speedup(infps, outfps);
	let path = output.join(&stream.id).with_extension("speed.w64");

	logging::info!("Changing speed by {}", speedup);

	let mut args = Vec::<&str>::new();

	args.push("-i");
	args.push(stream.path.to_str().unwrap());

	let map = format!("0:{}", stream.index);
	args.push("-map");
	args.push(&map);

	// Convert audio to original speed (no pitch correction)
	let asetrate = utils::speedup(infps, recfps);

	// Convert audio to target speed (with pitch correction)
	let atempo = utils::speedup(recfps, outfps);

	let af = format!(
		"asetrate={}*{},aresample,atempo={}",
		stream.samplerate.unwrap(),
		asetrate,
		atempo
	);
	args.push("-af");
	args.push(&af);

	let samplerate = stream.samplerate.unwrap().to_string();
	args.push("-ar");
	args.push(&samplerate);

	args.push("-resampler");
	args.push("soxr");

	args.push("-codec");
	args.push("pcm_f32le");

	args.push("-y");
	args.push(path.to_str().unwrap());

	let cmd = Command::new("ffmpeg")
		.args(args)
		.execute_check_exit_status_code(0);

	if let Err(err) = cmd {
		logging::error!("Failed to run ffmpeg: {}", err);
		return Err(());
	}

	let mut new = stream.clone();
	new.path = path.clone();
	new.index = 0;
	new.codec = Some(String::from("pcm_f32le"));
	new.offset /= speedup;
	new.duration /= speedup;

	Ok(new)
}

pub fn change_subtitles(
	stream: &mkv::Stream,
	output: &Path,
	infps: (u32, u32),
	outfps: (u32, u32),
) -> Result<mkv::Stream, ()> {
	let speedup = utils::speedup(infps, outfps);
	let path = output.join(&stream.id).with_extension("speed.mkv");

	logging::info!("Changing speed by {}", speedup);

	let mut args = Vec::<&str>::new();

	args.push("-i");
	args.push(stream.path.to_str().unwrap());

	let map = format!("0:{}", stream.index);
	args.push("-map");
	args.push(&map);

	let bsf = format!("setts=TS/{}", speedup);
	args.push("-bsf");
	args.push(&bsf);

	args.push("-codec");
	args.push("copy");

	args.push("-y");
	args.push(path.to_str().unwrap());

	let cmd = Command::new("ffmpeg")
		.args(args)
		.execute_check_exit_status_code(0);

	if let Err(err) = cmd {
		logging::error!("Failed to run ffmpeg: {}", err);
		return Err(());
	}

	let mut new = stream.clone();
	new.path = path.clone();
	new.index = 0;
	new.offset /= speedup;
	new.duration /= speedup;

	Ok(new)
}

pub fn change_chapters(
	stream: &mkv::Stream,
	output: &Path,
	infps: (u32, u32),
	outfps: (u32, u32),
) -> Result<mkv::Stream, ()> {
	let speedup = utils::speedup(infps, outfps);
	let path = output.join(&stream.id).with_extension("speed.txt");

	logging::info!("Changing speed by {}", speedup);

	let chapters = std::fs::read_to_string(&stream.path);
	if let Err(err) = chapters {
		logging::error!("Failed to read chapters: {}", err);
		return Err(());
	}

	let mut new = String::new();
	let chapters = chapters.unwrap();

	// Modify the timebase value to manipulate the timestamps
	// Changing the timebase changes start / stop at the same time
	for line in chapters.split('\n') {
		if line.starts_with("TIMEBASE") {
			let split = line.split('=').collect::<Vec<&str>>();
			let tb = utils::framerate(split[1]);

			let num = tb.0;
			let den = (tb.1 as f32 * speedup).round() as u32;

			new.push_str(format!("TIMEBASE={}/{}", num, den).as_str());
		} else {
			new.push_str(line);
		}

		new.push('\n');
	}

	let err = std::fs::write(&path, new);
	if let Err(err) = err {
		logging::error!("Failed to write chapters: {}", err);
		return Err(());
	}

	let mut new = stream.clone();
	new.path = path;
	new.offset /= speedup;
	new.duration /= speedup;

	Ok(new)
}
