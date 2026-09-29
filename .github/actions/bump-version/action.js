const core = require("./core");

// A release version without a prerelease part: MAJOR.MINOR.PATCH, no leading zeros.
const RELEASE_VERSION = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/;

// A semver prerelease: dot-separated identifiers of [0-9A-Za-z-], numeric ones without leading zeros.
const PRERELEASE = /^(0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*)(\.(0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*))*$/;

function bumpVersion(currentVersion, bumpType, prerelease) {
  if (!RELEASE_VERSION.test(currentVersion)) {
    throw new Error(
      `Invalid currentVersion: "${currentVersion}". Expected MAJOR.MINOR.PATCH, e.g. "7.0.0".`
    );
  }
  if (prerelease && !PRERELEASE.test(prerelease)) {
    throw new Error(
      `Invalid prereleaseSuffix: "${prerelease}". Use semver prerelease identifiers, e.g. "rc.1" or "beta1".`
    );
  }

  let newVersion = calculateNewVersion(currentVersion, bumpType);

  // Go module versions are semver, so the prerelease part follows a hyphen (v7.1.0-rc.1).
  if (prerelease) {
    newVersion = `${newVersion}-${prerelease}`;
  }
  core.setOutput("previous_version", currentVersion);
  core.setOutput("previous_version_tag", `v${currentVersion}`);
  core.setOutput("version", newVersion);
  core.setOutput("version_tag", `v${newVersion}`);
}

function calculateNewVersion(currentVersion, bumpType) {
  const [major, minor, patch] = currentVersion.split(".");
  let newVersion;

  switch (bumpType) {
    case "major":
      newVersion = `${parseInt(major) + 1}.0.0`;
      break;
    case "minor":
      newVersion = `${major}.${parseInt(minor) + 1}.0`;
      break;
    case "patch":
      newVersion = `${major}.${minor}.${parseInt(patch) + 1}`;
      break;
    default:
      throw new Error(`Invalid bumpType: ${bumpType}`);
  }

  return newVersion;
}

module.exports = { bumpVersion };
