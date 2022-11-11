use slog::{self, Drain};

pub use slog_scope::{crit, debug, error, info, trace, warn};

pub fn init() -> slog_scope::GlobalLoggerGuard {
	let dec = slog_term::TermDecorator::new().stdout().build();
	let drain = slog_term::CompactFormat::new(dec).build().fuse();
	let drain = slog_async::Async::new(drain).build().fuse();

	let root = slog::Logger::root(drain, slog::o!());
	slog_scope::set_global_logger(root)
}

pub fn scope<S, R>(key: &'static str, val: &str, func: S) -> R
where
	S: FnOnce() -> R,
{
	let root = slog_scope::logger();
	let logger = root.new(slog::o!(key => val.to_owned()));

	slog_scope::scope(&logger, func)
}
