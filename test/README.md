# RunQ Integration Tests

Some tests require a custom configuration file ([daemon.json](testdata/daemon.json))
and custom sigusr commands.

To install sigusr commands:
```
make -C testdata install
```

To run all tests:
```
make test
```

Some tests (e.g. disks.sh) require running as root or will be skipped.
To run all tests as root:
```
sudo -E make test
```

