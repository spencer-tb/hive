# This simulation feeds clients the Ethereum Execution Spec Tests as block rlp upon startup.
FROM python:3.10-slim

RUN apt-get update && \
    apt-get install -y git wget && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

RUN git clone --depth 1 --branch consume-rlp-prague https://github.com/spencer-tb/execution-spec-tests.git

WORKDIR /execution-spec-tests

# POINT to latest Prague release fixutures.
RUN wget https://github.com/ethereum/execution-spec-tests/files/15344998/fixtures_develop.tar.gz 
RUN tar -xzvf fixtures_develop.tar.gz

RUN find fixtures/ -name '._*' -exec rm '{}' \;
RUN rm -rf fixtures/assets/ fixtures/report_fill.html fixtures/blockchain_tests_hive fixtures/state_tests
RUN ls -a fixtures/
RUN ls -a fixtures/blockchain_tests

RUN pip install --upgrade pip
RUN pip install -e .

ENTRYPOINT ["pytest", "-c", "src/pytest_plugins/consume/ini_files/pytest-consume-rlp.ini", "--input", "fixtures/blockchain_tests/", "-v"]
