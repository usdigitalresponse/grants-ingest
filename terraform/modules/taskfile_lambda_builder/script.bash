#!/bin/bash

# Exit if any steps fail
set -e

eval "$(jq -r '@sh "TASK_COMMAND=\(.task_command)"')"
task $TASK_COMMAND >&2 && echo '{}'
