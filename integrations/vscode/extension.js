const vscode = require("vscode");
const { spawn } = require("child_process");

let channel;

function activate(context) {
  channel = vscode.window.createOutputChannel("Envdoctor");
  context.subscriptions.push(channel);
  context.subscriptions.push(vscode.commands.registerCommand("envdoctor.diagnose", () => runEnvdoctor(["diagnose", "--json"])));
  context.subscriptions.push(vscode.commands.registerCommand("envdoctor.fixPlan", () => runEnvdoctor(["fix", "plan", "--json", workspaceDir()])));
  context.subscriptions.push(vscode.commands.registerCommand("envdoctor.agentPlan", () => {
    const config = vscode.workspace.getConfiguration("envdoctor");
    const goal = config.get("goal", "diagnose");
    const profile = config.get("profile", "development");
    runEnvdoctor(["agent", "plan", "--json", "--goal", goal, "--profile", profile, workspaceDir()]);
  }));
}

function deactivate() {}

function runEnvdoctor(args) {
  const config = vscode.workspace.getConfiguration("envdoctor");
  const binary = config.get("path", "envdoctor");
  const cwd = workspaceDir();
  channel.clear();
  channel.show(true);
  channel.appendLine(`$ ${binary} ${args.join(" ")}`);
  const child = spawn(binary, args, { cwd, shell: false });
  child.stdout.on("data", (data) => channel.append(data.toString()));
  child.stderr.on("data", (data) => channel.append(data.toString()));
  child.on("error", (error) => {
    channel.appendLine(`Envdoctor failed to start: ${error.message}`);
  });
  child.on("close", (code) => {
    channel.appendLine("");
    channel.appendLine(`Envdoctor exited with code ${code}`);
  });
}

function workspaceDir() {
  const folders = vscode.workspace.workspaceFolders;
  if (folders && folders.length > 0) {
    return folders[0].uri.fsPath;
  }
  return process.cwd();
}

module.exports = {
  activate,
  deactivate,
};
