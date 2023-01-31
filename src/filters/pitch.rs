use std::{path::Path, process::Command};

use execute::Execute;

use crate::{
	logging, mkv,
	utils::{self, StrVec},
};

pub fn change(
	stream: &mkv::Stream,
	output: &Path,
	infps: (u32, u32),
	outfps: (u32, u32),
) -> Result<mkv::Stream, ()> {
	let speedup = utils::speedup(infps, outfps);
	let path = output.join(&stream.id).with_extension("pitch.w64");

	logging::info!("Changing pitch by {}", speedup);

	let mut args = Vec::<String>::new();

	args.push_str("-i");
	args.push_str(stream.path.to_str().unwrap());

	args.push_str("-map");
	args.push(format!("0:{}", stream.index));

	let af = format!(
		"asetrate={}*{},aresample,atempo=1/{}",
		stream.samplerate.unwrap(),
		speedup,
		speedup
	);
	args.push_str("-af");
	args.push(af);

	args.push_str("-ar");
	args.push(format!("{}", stream.samplerate.unwrap()));

	args.push_str("-resampler");
	args.push_str("soxr");

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
