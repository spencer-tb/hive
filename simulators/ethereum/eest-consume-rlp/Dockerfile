# ---------------------------------------------------------------#
# This simulation runs the EEST consume rlp simulator.           #
# ---------------------------------------------------------------#

FROM python:3.10-slim

# Install git and wget
RUN apt-get update && \
    apt-get install -y git wget && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Install execution-spec-tests
RUN git clone -b verkle/pydantic-rebase https://github.com/spencer-tb/execution-spec-tests.git
WORKDIR /execution-spec-tests
RUN pip install -e .

ENTRYPOINT ["consume", "rlp", "--input", "https://github.com/spencer-tb/execution-spec-tests/releases/tag/verkle-transition-v0/fixtures.tar.gz"]