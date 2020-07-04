# nrfbazel

This contains a collection of tools used for working with the nrf52 SDK with 
(Bazel)[https://bazel.build].

## nrfbazelify

I created this tool to make converting the nrf52 SDK to Bazel easier. It is
*not* meant to flawlessly generate C/C++ BUILD files for every scenario. I 
take advantage of many aspects of the nrf52 SDK that make my life easier. 
I also took a few shortcuts that might result in incorrect but decently close
output, and the rest will be done by hand.

Some of these assumptions include:

* We *DELETE* all BUILD files in the SDK tree. This is meant to convert a 
  fresh SDK to Bazel, not meant for helping to maintain it.
* We expect a matching .c file for every .h file - e.g. if the src file is
  named nrf_log_ctrl.c, and the header is named nrf_log.h, it won't match up the two.

### Usage

```bash
bazel run //:nrfbazelify -- --workspace /your/repo/abs/path --sdk /your/repo/abs/path/nrf_sdk_dir
```

This will delete all BUILD files in /your/repo/abs/path/nrf_sdk_dir, and generate new ones.

### Handling Unresolved Dependencies

