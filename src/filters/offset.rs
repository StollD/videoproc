use std::path::Path;

use crate::{logging, mkv};

pub fn change(stream: &mkv::Stream, output: &Path, seconds: f32) -> Result<mkv::Stream, ()> {
	logging::info!("Changing offset of stream {} by {}s", stream.id, seconds);

	let path = output.join(&stream.id).with_extension(format!(
		"offset.{}",
		stream.path.extension().unwrap().to_str().unwrap()
	));

	let err = std::fs::copy(&stream.path, &path);
	if let Err(err) = err {
		logging::error!("Failed to copy file: {}", err);
		return Err(());
	}

	let mut new = stream.clone();
	new.path = path;
	new.offset += seconds;

	Ok(new)
}
