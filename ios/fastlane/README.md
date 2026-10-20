fastlane documentation
----

# Installation

Make sure you have the latest version of the Xcode command line tools installed:

```sh
xcode-select --install
```

For _fastlane_ installation instructions, see [Installing _fastlane_](https://docs.fastlane.tools/#installing-fastlane)

# Available Actions

## iOS

### ios test

```sh
[bundle exec] fastlane ios test
```

Run unit tests on the simulator

### ios ci_build_number

```sh
[bundle exec] fastlane ios ci_build_number
```

Print next TestFlight build number for VERSION_NAME to GITHUB_OUTPUT

### ios prepare_signing

```sh
[bundle exec] fastlane ios prepare_signing
```

Provision code signing from the ASC API key into a persistent keychain

### ios build_ipa

```sh
[bundle exec] fastlane ios build_ipa
```

Archive and export a signed App Store IPA

### ios beta

```sh
[bundle exec] fastlane ios beta
```

Upload the built IPA to TestFlight (internal testers only)

### ios ensure_not_in_review

```sh
[bundle exec] fastlane ios ensure_not_in_review
```

Refuse to resubmit while a review submission is waiting or in review

### ios ensure_free_worldwide

```sh
[bundle exec] fastlane ios ensure_free_worldwide
```

Bootstrap free price schedule + all territories via current ASC APIs

### ios ensure_app_privacy

```sh
[bundle exec] fastlane ios ensure_app_privacy
```

Declare App Privacy as Data Not Collected (matches privacy policy)

### ios release

```sh
[bundle exec] fastlane ios release
```

Submit for App Store review — free, all territories, auto-release on approval

----

This README.md is auto-generated and will be re-generated every time [_fastlane_](https://fastlane.tools) is run.

More information about _fastlane_ can be found on [fastlane.tools](https://fastlane.tools).

The documentation of _fastlane_ can be found on [docs.fastlane.tools](https://docs.fastlane.tools).
