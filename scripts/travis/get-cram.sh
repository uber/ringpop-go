#!/bin/bash
set -e

if [[ ! -f "_venv/bin/activate" ]]; then
    virtualenv _venv
fi

. _venv/bin/activate

if [[ -z "$(which cram)" ]]; then
    ./_venv/bin/pip install cram
fi
