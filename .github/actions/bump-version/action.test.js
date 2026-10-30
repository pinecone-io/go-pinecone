// Run from the repo root: just test-actions
const { describe, test, beforeEach, mock } = require("node:test");
const assert = require("node:assert/strict");

const action = require("./action");
const core = require("./core");

function outputs() {
  return Object.fromEntries(
    core.setOutput.mock.calls.map((call) => call.arguments)
  );
}

function expectOutputs(expected) {
  const actual = outputs();
  for (const [name, value] of Object.entries(expected)) {
    assert.equal(actual[name], value, `output ${name}`);
  }
}

describe("bump-version", () => {
  beforeEach(() => {
    mock.restoreAll();
    mock.method(core, "setOutput", () => {});
  });

  test("bump major", () => {
    action.bumpVersion("1.2.3", "major", "");

    expectOutputs({
      previous_version: "1.2.3",
      previous_version_tag: "v1.2.3",
      version: "2.0.0",
      version_tag: "v2.0.0",
    });
  });

  test("bump minor: existing minor and patch", () => {
    action.bumpVersion("1.2.3", "minor", "");

    expectOutputs({
      previous_version: "1.2.3",
      previous_version_tag: "v1.2.3",
      version: "1.3.0",
      version_tag: "v1.3.0",
    });
  });

  test("bump minor: with no patch", () => {
    action.bumpVersion("1.2.0", "minor", "");

    expectOutputs({
      previous_version: "1.2.0",
      previous_version_tag: "v1.2.0",
      version: "1.3.0",
      version_tag: "v1.3.0",
    });
  });

  test("bump minor: from existing patch", () => {
    action.bumpVersion("2.2.3", "minor", "");

    expectOutputs({
      previous_version: "2.2.3",
      previous_version_tag: "v2.2.3",
      version: "2.3.0",
      version_tag: "v2.3.0",
    });
  });

  test("bump patch: existing patch", () => {
    action.bumpVersion("1.2.3", "patch", "");

    expectOutputs({
      previous_version: "1.2.3",
      previous_version_tag: "v1.2.3",
      version: "1.2.4",
      version_tag: "v1.2.4",
    });
  });

  test("bump patch: minor with no patch", () => {
    action.bumpVersion("1.2.0", "patch", "");

    expectOutputs({
      previous_version: "1.2.0",
      previous_version_tag: "v1.2.0",
      version: "1.2.1",
      version_tag: "v1.2.1",
    });
  });

  test("bump patch: major with no minor or patch", () => {
    action.bumpVersion("1.0.0", "patch", "");

    expectOutputs({
      previous_version: "1.0.0",
      previous_version_tag: "v1.0.0",
      version: "1.0.1",
      version_tag: "v1.0.1",
    });
  });

  test("bump patch: major with minor", () => {
    action.bumpVersion("1.1.0", "patch", "");

    expectOutputs({
      previous_version: "1.1.0",
      previous_version_tag: "v1.1.0",
      version: "1.1.1",
      version_tag: "v1.1.1",
    });
  });

  test("prerelease suffix provided", () => {
    action.bumpVersion("1.2.3", "patch", "rc1");

    expectOutputs({
      previous_version: "1.2.3",
      previous_version_tag: "v1.2.3",
      version: "1.2.4-rc1",
      version_tag: "v1.2.4-rc1",
    });
  });

  test("dotted prerelease suffix", () => {
    action.bumpVersion("7.0.0", "minor", "rc.1");

    expectOutputs({
      version: "7.1.0-rc.1",
      version_tag: "v7.1.0-rc.1",
    });
  });

  for (const currentVersion of ["", "7.0", "7.0.0.1", "v7.0.0", "7.1.0-rc1", "alpha-v1", "07.0.0"]) {
    test(`rejects invalid currentVersion ${JSON.stringify(currentVersion)}`, () => {
      assert.throws(() => action.bumpVersion(currentVersion, "patch", ""), /Invalid currentVersion/);
    });
  }

  for (const prerelease of ["rc 1", "rc..1", ".rc1", "rc1.", "rc_1", "rc.01"]) {
    test(`rejects invalid prereleaseSuffix ${JSON.stringify(prerelease)}`, () => {
      assert.throws(() => action.bumpVersion("7.0.0", "patch", prerelease), /Invalid prereleaseSuffix/);
    });
  }
});
