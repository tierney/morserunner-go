#!/usr/bin/env python3

import argparse
import socket


def main() -> None:
    parser = argparse.ArgumentParser(description="Send a line-delimited command to the MorseRunner sidecar socket")
    parser.add_argument("command", help="Command to send, e.g. 'pileup 3'")
    parser.add_argument("--socket", default="/tmp/morserunner.sock", help="Unix socket path")
    args = parser.parse_args()

    with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as sock:
        sock.connect(args.socket)
        sock.sendall((args.command + "\n").encode("utf-8"))


if __name__ == "__main__":
    main()
