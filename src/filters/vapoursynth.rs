use std::{
	path::Path,
	process::{Command, Stdio},
};

use execute::Execute;

use crate::{logging, mkv, utils};

pub fn run(stream: &mkv::Stream, output: &Path, filter: &Path) -> Result<mkv::Stream, ()> {
	if !filter.exists() {
		logging::error!("Filter not found!");
		return Err(());
	}

	let name = filter.file_stem().unwrap().to_str().unwrap();
	logging::info!("Filtering stream using {}", name);

	let script = output.join(format!("{}.{}.vpy", stream.id, name));
	let path = script.with_extension("vpy.mkv");

	let mpg = script.with_extension("vpy.mpg");
	let d2v = script.with_extension("vpy.d2v");

	let template = std::fs::read_to_string(filter);
	if let Err(err) = template {
		logging::error!("Failed to read filter: {}", err);
		return Err(());
	}

	let mut template = template.unwrap();
	template = template.replace("$(mkv)$", stream.path.to_str().unwrap());
	template = template.replace("$(vpy)$", filter.to_str().unwrap());

	// If requested create a D2V Index for MPEG2 streams
	if template.contains("$(d2v)$") {
		let cmd = Command::new("mkvextract")
			.arg(stream.path.to_str().unwrap())
			.arg("tracks")
			.arg(format!("0:{}", &mpg.to_str().unwrap()))
			.execute_check_exit_status_code(0);

		if let Err(err) = cmd {
			logging::error!("Failed to extract video stream: {}", err);
			return Err(());
		}

		let cmd = Command::new("d2vwitch")
			.arg("--output")
			.arg(d2v.to_str().unwrap())
			.arg(mpg.to_str().unwrap())
			.execute_check_exit_status_code(0);

		if let Err(err) = cmd {
			logging::error!("Failed to create D2V index: {}", err);
			return Err(());
		}

		template = template.replace("$(d2v)$", d2v.to_str().unwrap());
	}

	let err = std::fs::write(&script, template);
	if let Err(err) = err {
		logging::error!("Failed to write vapoursynth script: {}", err);
		return Err(());
	}

	let vspipe = Command::new("vspipe")
		.arg(script.to_str().unwrap())
		.arg("-")
		.arg("-c")
		.arg("y4m")
		.stdout(Stdio::piped())
		.stderr(Stdio::null())
		.spawn();

	if let Err(err) = vspipe {
		logging::error!("Failed to run vspipe: {}", err);
		return Err(());
	}

	let mut vspipe = vspipe.unwrap();

	let mut args = vec!["-i", "pipe:", "-codec", "ffv1", "-map", "0"];

	if stream.aspect.is_some() {
		args.push("-aspect");
		args.push(stream.aspect.as_deref().unwrap());
	}

	args.push("-y");
	args.push(path.to_str().unwrap());

	let ffmpeg = Command::new("ffmpeg")
		.args(args)
		.stdin(Stdio::from(vspipe.stdout.take().unwrap()))
		.output();

	let ffmpeg = utils::check_output(ffmpeg);
	if let Err(err) = ffmpeg {
		let _ = vspipe.kill();
		logging::error!("Failed to run ffmpeg: {}", err);
		return Err(());
	}

	let probe = Command::new("vspipe").arg("-i").arg(&script).output();

	if let Err(err) = probe {
		logging::error!("Failed to run vspipe: {}", err);
		return Err(());
	}

	let probe = probe.unwrap();
	let probe = String::from_utf8(probe.stdout);
	if let Err(err) = probe {
		logging::error!("Failed to decode vspipe output: {}", err);
		return Err(());
	}

	let probe = probe.unwrap();
	let mut frames = 0;
	let mut framerate = (0u32, 0u32);

	for line in probe.split('\n') {
		let split = line.split(':').collect::<Vec<&str>>();

		if split[0] == "Frames" {
			let fr = split[1].trim().parse::<u32>();
			if let Err(err) = fr {
				logging::error!("Failed to parse frame count: {}", err);
				return Err(());
			}

			frames = fr.unwrap();
		}

		if split[0] == "FPS" {
			let split = split[1].trim().split(' ').collect::<Vec<&str>>();
			framerate = utils::framerate(split[0]);
		}
	}

	let duration = frames as f32 / (framerate.0 as f32 / framerate.1 as f32);
	let speedup = stream.duration / duration;

	let err = std::fs::remove_file(script);
	if let Err(err) = err {
		logging::error!("Failed to remove file: {}", err);
		return Err(());
	}

	if mpg.exists() {
		let err = std::fs::remove_file(mpg);
		if let Err(err) = err {
			logging::error!("Failed to remove file: {}", err);
			return Err(());
		}
	}

	if d2v.exists() {
		let err = std::fs::remove_file(d2v);
		if let Err(err) = err {
			logging::error!("Failed to remove file: {}", err);
			return Err(());
		}
	}

	let mut new = stream.clone();
	new.path = path;
	new.index = 0;
	new.codec = Some(String::from("ffv1"));
	new.offset /= speedup;
	new.duration /= speedup;

	Ok(new)
}
