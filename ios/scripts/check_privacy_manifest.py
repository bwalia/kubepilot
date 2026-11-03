#!/usr/bin/env python3
"""Check the iOS privacy manifest against what the code actually does.

    python3 ios/scripts/check_privacy_manifest.py

Apple requires a reason declared for every "required reason API" a binary calls
(mandatory since 1 May 2024). Adding one of those calls is a one-line change in
Swift that nobody connects to a plist — so the build uploads, Apple answers with
ITMS-91053 or a privacy rejection, and the round trip costs days. This greps the
sources for those APIs and fails if the manifest does not cover them.

It also pins the three statements that have to agree, because App Review reads
all three and treats any mismatch as the answer:
  this manifest  ==  ASC App Privacy "Data Not Collected"  ==  privacy.html

Exits non-zero on any mismatch.
"""
import plistlib
import re
import sys
from pathlib import Path

IOS = Path(__file__).resolve().parent.parent
MANIFEST = IOS / "KubePilot/PrivacyInfo.xcprivacy"
INFO_PLIST = IOS / "KubePilot/Info.plist"
SOURCES = [IOS / "KubePilot", IOS / "KubePilotWidgets"]

# Apple's five required-reason categories, and the Swift that gives each away.
# Keep the patterns broad: a false alarm costs one line in the manifest, a miss
# costs an App Store rejection.
REQUIRED_REASON_APIS = {
    "NSPrivacyAccessedAPICategoryUserDefaults": [
        r"\bUserDefaults\b", r"@AppStorage\b",
    ],
    "NSPrivacyAccessedAPICategoryFileTimestamp": [
        r"\.contentModificationDate\b", r"\.creationDate\b",
        r"\battributesOfItem\b", r"\bgetattrlist\b", r"\.fileModificationDate\b",
    ],
    "NSPrivacyAccessedAPICategorySystemBootTime": [
        r"\bsystemUptime\b", r"\bmach_absolute_time\b",
    ],
    "NSPrivacyAccessedAPICategoryDiskSpace": [
        r"\bvolumeAvailableCapacity\w*\b", r"\bsystemFreeSize\b", r"\bsystemSize\b",
    ],
    "NSPrivacyAccessedAPICategoryActiveKeyboards": [
        r"\bactiveInputModes\b",
    ],
}

# Keys that must stay in Info.plist. Each one was added for a reason that is not
# obvious from the key name, so the reason travels with the check.
REQUIRED_INFO_KEYS = {
    "ITSAppUsesNonExemptEncryption":
        "export-compliance answer; without it every TestFlight upload stalls",
    "NSAppTransportSecurity":
        "ATS blocks http:// servers on a LAN or in-cluster without this",
    "NSLocalNetworkUsageDescription":
        "no prompt, so a server on 10.x/192.168.x just fails to connect",
}


def swift_sources() -> list[Path]:
    return [p for root in SOURCES for p in root.rglob("*.swift")]


def used_categories(files: list[Path]) -> dict[str, tuple[str, int]]:
    """Map each required-reason category to the first place it is used."""
    found: dict[str, tuple[str, int]] = {}
    for path in files:
        try:
            lines = path.read_text(encoding="utf-8").splitlines()
        except (OSError, UnicodeDecodeError):
            continue
        for n, line in enumerate(lines, 1):
            if line.lstrip().startswith("//"):
                continue
            for category, patterns in REQUIRED_REASON_APIS.items():
                if category in found:
                    continue
                if any(re.search(p, line) for p in patterns):
                    found[category] = (str(path.relative_to(IOS)), n)
    return found


def main() -> int:
    problems: list[str] = []

    if not MANIFEST.exists():
        print(f"FAIL: no privacy manifest at {MANIFEST.relative_to(IOS)}")
        return 1
    manifest = plistlib.loads(MANIFEST.read_bytes())

    # 1. The manifest must keep saying "nothing collected, nobody tracked", or it
    #    no longer matches the ASC answer and the published privacy policy.
    if manifest.get("NSPrivacyTracking") is not False:
        problems.append("NSPrivacyTracking must be false (the app has no tracking SDK)")
    for key in ("NSPrivacyTrackingDomains", "NSPrivacyCollectedDataTypes"):
        if manifest.get(key, []) != []:
            problems.append(
                f"{key} is no longer empty — ASC App Privacy says Data Not Collected "
                "and privacy.html agrees, so update all three together or none"
            )

    # 2. Every required-reason API the code calls must have a declared reason.
    declared = {
        entry.get("NSPrivacyAccessedAPIType"): entry.get("NSPrivacyAccessedAPITypeReasons") or []
        for entry in manifest.get("NSPrivacyAccessedAPITypes", [])
    }
    for category, (path, line) in sorted(used_categories(swift_sources()).items()):
        if category not in declared:
            problems.append(
                f"{category} is used at {path}:{line} but not declared — "
                "Apple answers this with ITMS-91053"
            )
        elif not declared[category]:
            problems.append(f"{category} is declared with no reason code")

    # 3. Declaring a category nothing calls is dead weight, and Apple asks about it.
    for category in declared:
        if category in REQUIRED_REASON_APIS and category not in used_categories(swift_sources()):
            problems.append(f"{category} is declared but nothing in the sources uses it")

    # 4. The Info.plist keys that review and ATS depend on.
    if not INFO_PLIST.exists():
        problems.append(f"{INFO_PLIST.relative_to(IOS)} is missing — run `xcodegen generate`")
    else:
        info = plistlib.loads(INFO_PLIST.read_bytes())
        for key, why in REQUIRED_INFO_KEYS.items():
            if key not in info:
                problems.append(f"Info.plist lost {key} — {why}")

    for category, reasons in sorted(declared.items()):
        print(f"  ok    {category} ({', '.join(reasons)})")
    print(f"  ok    tracking=false, 0 tracking domains, 0 collected data types")

    if problems:
        print(f"\n{len(problems)} problem(s):")
        for p in problems:
            print(f"  FAIL  {p}")
        return 1
    print("\nprivacy manifest: all checks passed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
