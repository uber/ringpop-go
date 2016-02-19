.PHONY: clean clean-tests setup test out update-tests

out: test

clean: clean-tests

clean-tests:
	rm -vf test/*.err

setup:
	pip install cram

test:
	BASE_DIR="$$PWD" cram -v test/*.t

update-tests:
	BASE_DIR="$$PWD" cram -v -i test/*.t
