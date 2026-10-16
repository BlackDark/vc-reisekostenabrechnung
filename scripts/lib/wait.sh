# Shared readiness waits for the scripts that start containers.
#
# wait_until ATTEMPTS INTERVAL DESCRIPTION CMD... tries the command every
# INTERVAL seconds, at most ATTEMPTS times, and reports DESCRIPTION when it
# gives up. The command's output is discarded, so a caller that needs it runs
# the command again after the wait.

wait_until() {
	attempts=$1
	interval=$2
	description=$3
	shift 3
	i=0
	while [ "$i" -lt "$attempts" ]; do
		if "$@" >/dev/null 2>&1; then
			return 0
		fi
		i=$((i + 1))
		if [ "$i" -lt "$attempts" ]; then
			sleep "$interval"
		fi
	done
	echo "gave up after $attempts attempts waiting for $description" >&2
	return 1
}
