use std::io;

use crate::mkv;

pub fn framerate(input: &str) -> (u32, u32) {
	if input.contains('/') {
		let mut split = input.split('/');

		let num = split.next().unwrap().parse::<u32>().unwrap_or_default();
		let den = split.next().unwrap().parse::<u32>().unwrap_or_default();

		(num, den)
	} else {
		let num = input.parse::<u32>().unwrap_or_default();
		(num, 1)
	}
}

pub fn streampriority(stream: &mkv::Stream) -> u32 {
	if stream.streamtype == "video" {
		1
	} else if stream.streamtype == "audio" {
		2
	} else if stream.streamtype == "subtitle" {
		3
	} else {
		4
	}
}

pub fn speedup(infps: (u32, u32), outfps: (u32, u32)) -> f32 {
	(outfps.0 as f32 / outfps.1 as f32) / (infps.0 as f32 / infps.1 as f32)
}

pub fn check_output(
	cmd: Result<std::process::Output, io::Error>,
) -> Result<std::process::Output, io::Error> {
	let cmd = cmd?;

	if !cmd.status.success() {
		return Err(io::Error::new(
			io::ErrorKind::Other,
			format!(
				"Command exited with code {}",
				cmd.status.code().unwrap_or(-1)
			),
		));
	}

	Ok(cmd)
}

pub trait StrVec {
	fn push_str(&mut self, value: &str);
}

impl StrVec for Vec<String> {
	fn push_str(&mut self, value: &str) {
		self.push(String::from(value));
	}
}
