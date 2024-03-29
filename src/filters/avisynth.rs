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
	logging::info!("Filtering stream using AviSynth filter {}", name);

	let script = output.join(format!("{}.{}.avs", stream.id, name));
	let path = script.with_extension("avs.mkv");

	let mpg = script.with_extension("avs.mpg");
	let d2v = script.with_extension("avs.d2v");

	let template = std::fs::read_to_string(filter);
	if let Err(err) = template {
		logging::error!("Failed to read filter: {}", err);
		return Err(());
	}

	let mut template = template.unwrap();
	template = template.replace("$(mkv)$", stream.path.to_str().unwrap());
	template = template.replace("$(avs)$", filter.to_str().unwrap());

	// If requested extract the MPEG-2 stream
	if template.contains("$(d2v)$") || template.contains("$(mpg)$") {
		let cmd = Command::new("mkvextract")
			.arg(stream.path.to_str().unwrap())
			.arg("tracks")
			.arg(format!("0:{}", &mpg.to_str().unwrap()))
			.execute_check_exit_status_code(0);

		if let Err(err) = cmd {
			logging::error!("Failed to extract video stream: {}", err);
			return Err(());
		}

		template = template.replace("$(mpg)$", mpg.to_str().unwrap());
	}

	// If requested create a D2V Index for MPEG2 streams
	if template.contains("$(d2v)$") {
		let mpg_name = mpg.file_name().unwrap();
		let project = d2v.with_extension("");
		let project_name = project.file_name().unwrap();

		let cmd = Command::new("DGIndex")
			.current_dir(mpg.parent().unwrap())
			.arg("-i")
			.arg(mpg_name.to_str().unwrap())
			.arg("-o")
			.arg(project_name.to_str().unwrap())
			.arg("-exit")
			.arg("-hide")
			.execute_check_exit_status_code(0);

		if let Err(err) = cmd {
			logging::error!("Failed to create D2V index: {}", err);
			return Err(());
		}

		template = template.replace("$(d2v)$", d2v.to_str().unwrap());
	}

	// Is this a two pass script?
	if template.contains("$(pass)$") {
		let script = script.with_extension("pass1.avs");
		let p1 = template.replace("$(pass)$", "1");

		let err = std::fs::write(&script, p1);
		if let Err(err) = err {
			logging::error!("Failed to write avisynth script: {}", err);
			return Err(());
		}

		let avspipe = Command::new("avs2yuv")
			.current_dir(script.parent().unwrap())
			.arg(script.to_str().unwrap())
			.arg("-o")
			.arg("-")
			.execute_check_exit_status_code(0);

		if let Err(err) = avspipe {
			logging::error!("Failed to run avs2yuv: {}", err);
			return Err(());
		}

		let err = std::fs::remove_file(script);
		if let Err(err) = err {
			logging::error!("Failed to remove file: {}", err);
			return Err(());
		}

		template = template.replace("$(pass)$", "2");
	}

	let err = std::fs::write(&script, template);
	if let Err(err) = err {
		logging::error!("Failed to write avisynth script: {}", err);
		return Err(());
	}

	let avspipe = Command::new("avs2yuv")
		.current_dir(script.parent().unwrap())
		.arg(script.to_str().unwrap())
		.arg("-o")
		.arg("-")
		.stdout(Stdio::piped())
		.stderr(Stdio::null())
		.spawn();

	if let Err(err) = avspipe {
		logging::error!("Failed to run avs2yuv: {}", err);
		return Err(());
	}

	let mut avspipe = avspipe.unwrap();

	let mut args = vec!["-i", "pipe:", "-codec", "ffv1", "-map", "0"];

	if stream.aspect.is_some() {
		args.push("-aspect");
		args.push(stream.aspect.as_deref().unwrap());
	}

	args.push("-y");
	args.push(path.to_str().unwrap());

	let ffmpeg = Command::new("ffmpeg")
		.current_dir(script.parent().unwrap())
		.args(args)
		.stdin(Stdio::from(avspipe.stdout.take().unwrap()))
		.output();

	let ffmpeg = utils::check_output(ffmpeg);
	if let Err(err) = ffmpeg {
		let _ = avspipe.kill();
		logging::error!("Failed to run ffmpeg: {}", err);
		return Err(());
	}

	let probe = mkv::stream(&path, 0);

	if probe.is_err() {
		logging::error!("Failed to probe output file!");
		return Err(());
	}

	let probe = probe.unwrap();
	let speedup = stream.duration / probe.duration;

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
