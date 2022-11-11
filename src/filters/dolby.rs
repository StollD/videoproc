use execute::Execute;
use std::{path::Path, process::Command};

use crate::{logging, mkv};

pub fn normalize(stream: &mkv::Stream, output: &Path) -> Result<mkv::Stream, ()> {
	logging::info!("Normalizing AC3 audio");

	let path = output.join(&stream.id).with_extension("norm.w64");

	let mut args = Vec::<&str>::new();

	args.push("-drc_scale");
	args.push("0");

	let mapping = format!("0:{}", stream.index);
	let dialnorm = stream.dialnorm.map(|d| d.to_string());

	if dialnorm.is_some() {
		args.push("-target_level");
		args.push(dialnorm.as_deref().unwrap());
	}

	args.push("-i");
	args.push(stream.path.to_str().unwrap());
	args.push("-map");
	args.push(&mapping);
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

	Ok(new)
}
