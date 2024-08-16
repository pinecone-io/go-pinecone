const action = require("./action");
const core = require("./core");

action.bumpVersion(
  core.getInput("currentVersion"),
  core.getInput("bumpType"),
  core.getInput("prereleaseSuffix")
);
