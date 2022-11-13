use execute::Execute;
use std::{path::Path, process::Command};

use crate::{logging, mkv, utils::StrVec};

pub fn normalize(stream: &mkv::Stream, output: &Path) -> Result<mkv::Stream, ()> {
	logging::info!("Normalizing AC3 audio");

	let path = output.join(&stream.id).with_extension("norm.w64");

	let mut args = Vec::<String>::new();

	args.push_str("-drc_scale");
	args.push_str("0");

	if stream.dialnorm.is_some() {
		args.push_str("-target_level");
		args.push(format!("{}", stream.dialnorm.unwrap()));
	}

	args.push_str("-i");
	args.push_str(stream.path.to_str().unwrap());
	args.push_str("-map");
	args.push(format!("0:{}", stream.index));
	args.push_str("-codec");
	args.push_str("pcm_f32le");
	args.push_str("-y");
	args.push_str(path.to_str().unwrap());

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
