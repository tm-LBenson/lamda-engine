using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Drawing;
using System.IO;
using System.Linq;
using System.Management;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using System.Web.Script.Serialization;
using System.Windows.Forms;

namespace LamdaUI {
    internal static class Files {
        public static Dictionary<string, object> Json(string path) {
            using (var file = new FileStream(path, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete)) {
                if (file.Length > 1024 * 1024) throw new IOException("Status file is too large.");
                using (var reader = new StreamReader(file, Encoding.UTF8))
                    return new JavaScriptSerializer().Deserialize<Dictionary<string, object>>(reader.ReadToEnd());
            }
        }
        public static string Text(Dictionary<string, object> value, string key) {
            object item;
            return value != null && value.TryGetValue(key, out item) ? Convert.ToString(item) : "";
        }
        public static string Full(string value) { return Path.GetFullPath(value).TrimEnd('\\', '/'); }
        public static bool Same(string a, string b) {
            return !String.IsNullOrEmpty(a) && !String.IsNullOrEmpty(b) && String.Equals(Full(a), Full(b), StringComparison.OrdinalIgnoreCase);
        }
        public static string Quote(string value) {
            if (value.IndexOfAny(new char[] { '"', '\r', '\n' }) >= 0 || value.EndsWith("\\")) throw new ArgumentException("Invalid launch path.");
            return "\"" + value + "\"";
        }
    }

    internal sealed class Client {
        public readonly string Key, Name, Home, Game;
        public string Exe { get { return Path.Combine(Home, "lamda-engine.exe"); } }
        public string State { get { return Path.Combine(Home, "data", "state.json"); } }
        public string GameExecutable { get { return Key == "forever" && (String.IsNullOrWhiteSpace(Game) || !String.Equals(Path.GetFileName(Files.Full(Game)), "_forever_", StringComparison.OrdinalIgnoreCase)) ? "WowB.exe" : "Wow.exe"; } }
        public Client(string root, string key, string game) {
            if (key != "retail" && key != "forever") throw new ArgumentException("Unknown client.");
            Key = key; Name = key == "retail" ? "Retail" : "Forever";
            Home = Path.Combine(root, Name); Game = game;
        }
    }

    internal sealed class EngineState {
        public bool Installed, Running, Healthy, Unknown, GameOpen;
        public int Pid;
        public string Version = "", AddonVersion = "", Headline = "Stopped", Detail = "", Input = "";
        public Color Color = Color.FromArgb(148, 164, 182);
        public static EngineState Classify(bool installed, bool unknown, int pid, bool gameOpen,
                                           string version, Dictionary<string, object> state, DateTime modified, DateTime now, string client) {
            var result = new EngineState { Installed = installed, Unknown = unknown, Running = pid > 0, Pid = pid, GameOpen = gameOpen, Version = version };
            int reported;
            result.Healthy = pid > 0 && Int32.TryParse(Files.Text(state, "pid"), out reported) && reported == pid &&
                Files.Text(state, "client") == client && modified <= now.AddSeconds(5) && modified >= now.AddSeconds(-15);
            result.Input = Files.Text(state, "inputStatus");
            if (unknown) { result.Headline = "Status unavailable"; result.Detail = "Unable to verify the companion process."; result.Color = Palette.Amber; }
            else if (!installed) { result.Headline = "Not installed"; result.Detail = "This client needs a managed LamdaUI installation."; }
            else if (!result.Running) { result.Headline = "Engine stopped"; result.Detail = gameOpen ? "WoW is open. Start the companion for team estimates." : "Start the companion when you are ready to play."; }
            else if (!result.Healthy) { result.Headline = "Waiting for status"; result.Detail = "The process is running; no recent matching status yet."; result.Color = Palette.Amber; }
            else { result.Headline = "Engine running"; result.Detail = gameOpen ? "WoW is open. Companion is responding." : "Ready when you launch WoW."; result.Color = Palette.Accent; }
            return result;
        }
    }

    internal sealed class Gate : IDisposable {
        public const string Name = @"Local\LamdaUI.EngineControl";
        private Mutex mutex;
        private bool owns;
        public bool Acquired { get { return owns; } }
        public Gate(string name = Name) {
            mutex = new Mutex(false, name);
            try { owns = mutex.WaitOne(0); } catch (AbandonedMutexException) { owns = true; }
        }
        public void Dispose() {
            if (owns) { mutex.ReleaseMutex(); owns = false; }
            mutex.Dispose();
        }
        public static bool Busy(string name = Name) { using (var gate = new Gate(name)) return !gate.Acquired; }
    }

    internal sealed class Controller {
        public readonly string Root, Updates;
        public readonly Client[] Clients;
        public readonly bool Portable;
        private readonly Dictionary<string, string> stoppedArguments = new Dictionary<string, string>();
        public Controller(string root) {
            Root = Files.Full(root);
            string updaterConfig = Path.Combine(Root, "Updater", "updater.json");
            var config = Files.Json(updaterConfig);
            if (Files.Text(config, "product") != "LamdaUI") throw new IOException("LamdaUI updater configuration is missing.");
            Portable = Files.Text(config, "schema") == "2";
            if (Portable) {
                Updates = Path.Combine(Root, "Updater", "data");
                object configured;
                var games = config.TryGetValue("clients", out configured) ? configured as Dictionary<string, object> : null;
                if (games == null) throw new IOException("Open the LamdaUI installer to choose your WoW folder.");
                Clients = new [] { new Client(Root, "retail", Files.Text(games, "retail")), new Client(Root, "forever", Files.Text(games, "forever")) };
            } else {
                Updates = Path.Combine(Files.Text(config, "projectWindows"), ".build", "updates");
                string forever = File.Exists(@"D:\World of Warcraft\_forever_\Wow.exe") ? @"D:\World of Warcraft\_forever_" : @"D:\World of Warcraft\_classic_beta_";
                Clients = new [] { new Client(Root, "retail", @"D:\World of Warcraft\_retail_"), new Client(Root, "forever", forever) };
            }
        }
        public static List<Process> Matching(string name, string path, out bool uncertain) {
            uncertain = false;
            var result = new List<Process>();
            foreach (var process in Process.GetProcessesByName(name)) {
                try {
                    if (!process.HasExited && Files.Same(process.MainModule.FileName, path)) { result.Add(process); continue; }
                } catch (InvalidOperationException) { }
                catch (System.ComponentModel.Win32Exception) { uncertain = true; }
                process.Dispose();
            }
            return result;
        }
        public EngineState Read(Client client) {
            bool installed = false, uncertain, gameUnknown;
            string version = "", addon = "";
            try {
                var manifest = Files.Json(Path.Combine(client.Home, "manifest.json"));
                installed = Files.Text(manifest, "product") == "LamdaUI" && Files.Text(manifest, "client") == client.Key && File.Exists(client.Exe);
                version = Files.Text(manifest, "version");
                var toc = Path.Combine(client.Game, "Interface", "AddOns", "lamdaUI", "lamdaUI.toc");
                var line = File.ReadLines(toc).FirstOrDefault(s => s.StartsWith("## Version:"));
                addon = line == null ? "" : line.Substring(11).Trim();
            } catch (IOException) { } catch (UnauthorizedAccessException) { } catch (ArgumentException) { }
            var processes = Matching("lamda-engine", client.Exe, out uncertain);
            int pid = processes.Count == 1 ? processes[0].Id : 0;
            uncertain = uncertain || processes.Count > 1;
            foreach (var process in processes) process.Dispose();
            var games = Matching(Path.GetFileNameWithoutExtension(client.GameExecutable), Path.Combine(client.Game, client.GameExecutable), out gameUnknown);
            bool gameOpen = games.Count > 0;
            foreach (var game in games) game.Dispose();
            Dictionary<string, object> state = null;
            DateTime modified = DateTime.MinValue;
            try { state = Files.Json(client.State); modified = File.GetLastWriteTimeUtc(client.State); }
            catch (IOException) { } catch (ArgumentException) { } catch (UnauthorizedAccessException) { }
            var result = EngineState.Classify(installed, uncertain, pid, gameOpen, version, state, modified, DateTime.UtcNow, client.Key);
            result.AddonVersion = addon;
            return result;
        }
        private string ExistingArguments(Process process, Client client) {
            using (var query = new ManagementObject("Win32_Process.Handle='" + process.Id + "'")) {
                query.Options.Timeout = TimeSpan.FromSeconds(5);
                query.Get();
                if (!Files.Same(Convert.ToString(query["ExecutablePath"]), client.Exe)) throw new IOException("The companion process changed. Try again.");
                string command = Convert.ToString(query["CommandLine"]);
                string prefix = Files.Quote(client.Exe);
                if (command.StartsWith(prefix + " ", StringComparison.OrdinalIgnoreCase)) return command.Substring(prefix.Length).TrimStart();
                if (command.StartsWith(client.Exe + " ", StringComparison.OrdinalIgnoreCase)) return command.Substring(client.Exe.Length).TrimStart();
                throw new IOException("Could not preserve the selected account and launch options.");
            }
        }
        private void Stop(Client client, bool remember) {
            bool uncertain;
            var processes = Matching("lamda-engine", client.Exe, out uncertain);
            try {
                if (uncertain || processes.Count > 1) throw new IOException("Could not identify exactly one companion. Open logs before trying again.");
                if (processes.Count == 0) return;
                var process = processes[0];
                if (remember) stoppedArguments[client.Key] = ExistingArguments(process, client);
                if (!Files.Same(process.MainModule.FileName, client.Exe)) throw new IOException("The process changed.");
                process.Kill();
                if (!process.WaitForExit(10000)) throw new IOException("The companion has not stopped yet.");
            } finally { foreach (var process in processes) process.Dispose(); }
        }
        private void Start(Client client) {
            var status = Read(client);
            if (!status.Installed || status.Unknown) throw new IOException("The companion installation could not be verified.");
            if (status.Running) return;
            string arguments;
            if (stoppedArguments.TryGetValue(client.Key, out arguments)) {
                var info = new ProcessStartInfo(client.Exe, arguments) { UseShellExecute = false, CreateNoWindow = true, WorkingDirectory = client.Home };
                using (var process = Process.Start(info)) { }
            } else if (Portable) {
                if (String.IsNullOrEmpty(client.Game)) throw new IOException("Run the LamdaUI installer to add this game.");
                string data = Path.Combine(client.Home, "data"); Directory.CreateDirectory(data);
                arguments = "--client " + client.Key + " --wow " + Files.Quote(client.Game) + " --state-dir " + Files.Quote(data);
                string account = SelectedAccount(client);
                if (account != "") arguments += " --config " + Files.Quote(Path.Combine(client.Game, "WTF", "Account", account, "SavedVariables", "lamdaUI.lua"));
                using (var process = Process.Start(new ProcessStartInfo(client.Exe, arguments) { UseShellExecute = false, CreateNoWindow = true, WorkingDirectory = client.Home })) { }
            } else {
                var info = PowerShell(Path.Combine(client.Home, "Start.ps1"), "");
                info.RedirectStandardOutput = true; info.RedirectStandardError = true;
                using (var process = Process.Start(info)) {
                    Task<string> output = process.StandardOutput.ReadToEndAsync(), error = process.StandardError.ReadToEndAsync();
                    if (!process.WaitForExit(20000)) { process.Kill(); throw new IOException("Starting the companion took too long."); }
                    Task.WaitAll(output, error);
                    if (process.ExitCode != 0) throw new IOException((error.Result + " " + output.Result).Trim());
                }
            }
            for (int i = 0; i < 40; i++) {
                if (Read(client).Healthy) return;
                Thread.Sleep(250);
            }
            throw new IOException("The companion did not report a healthy startup. Open its logs for details.");
        }
        private string SelectedAccount(Client client) {
            try {
                string name = Files.Text(Files.Json(Path.Combine(client.Home, "data", "account.json")), "account");
                if (name != "" && name != "." && name != ".." && name.IndexOfAny(new [] {'\\', '/', '"', '\r', '\n'}) < 0 && Directory.Exists(Path.Combine(client.Game, "WTF", "Account", name))) return name;
            } catch (IOException) { } catch (ArgumentException) { }
            return "";
        }
        public bool ChooseAccount(Client client, IWin32Window owner) {
            if (!Portable || Read(client).Running || SelectedAccount(client) != "") return true;
            string folder = Path.Combine(client.Game, "WTF", "Account");
            var names = Directory.Exists(folder) ? Directory.GetDirectories(folder).Select(Path.GetFileName).Where(n => n != "SavedVariables").OrderBy(n => n).ToArray() : new string[0];
            if (names.Length < 2) return true; // The engine finds a single account after the first login/reload.
            using (var dialog = new Form { Text = "Choose your WoW account", ClientSize = new Size(450, 150), StartPosition = FormStartPosition.CenterParent, FormBorderStyle = FormBorderStyle.FixedDialog, MaximizeBox = false, MinimizeBox = false }) {
                dialog.Controls.Add(new Label { Text = "Which WoW account do you want to use?", Left = 20, Top = 20, Width = 410, Height = 25 });
                var choice = new ComboBox { Left = 20, Top = 53, Width = 410, DropDownStyle = ComboBoxStyle.DropDownList };
                foreach (string name in names) {
                    var characters = new List<string>();
                    foreach (string realm in Directory.GetDirectories(Path.Combine(folder, name))) {
                        if (Path.GetFileName(realm) == "SavedVariables") continue;
                        characters.AddRange(Directory.GetDirectories(realm).Select(Path.GetFileName).Take(3));
                    }
                    choice.Items.Add(characters.Count > 0 ? name + " — " + String.Join(", ", characters.Take(4)) : name);
                }
                choice.SelectedIndex = 0; dialog.Controls.Add(choice);
                var button = new Button { Text = "Continue", Left = 320, Top = 100, Width = 110, DialogResult = DialogResult.OK }; dialog.Controls.Add(button); dialog.AcceptButton = button;
                if (dialog.ShowDialog(owner) != DialogResult.OK) return false;
                string data = Path.Combine(client.Home, "data"); Directory.CreateDirectory(data);
                File.WriteAllText(Path.Combine(data, "account.json"), new JavaScriptSerializer().Serialize(new Dictionary<string, object> {{"account", names[choice.SelectedIndex]}}));
            }
            return true;
        }
        public void Execute(Client client, string action) {
            if (action != "start" && action != "stop" && action != "restart") throw new ArgumentException("Unknown action.");
            using (var gate = new Gate()) {
                if (!gate.Acquired) throw new IOException("The updater or another engine control is busy. Wait for it to finish.");
                if (action == "stop" || action == "restart") Stop(client, true);
                if (action == "start" || action == "restart") Start(client);
            }
        }
        public static ProcessStartInfo PowerShell(string file, string extra) {
            return new ProcessStartInfo(Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.System), @"WindowsPowerShell\v1.0\powershell.exe"),
                "-NoProfile -NonInteractive -ExecutionPolicy Bypass -File " + Files.Quote(file) + extra) { UseShellExecute = false, CreateNoWindow = true };
        }
        public static ProcessStartInfo OverlayWindowsCommand(Client client) {
            return PowerShell(Path.Combine(client.Home, "frame-estimates.ps1"),
                " -ManageWindows -StatePath " + Files.Quote(Path.Combine(client.Home, "data", "frame-display.json")));
        }
        public void OpenOverlayWindows(Client client) {
            if (!Read(client).Healthy) throw new IOException("Start the " + client.Name + " companion before opening overlay windows.");
            if (!File.Exists(Path.Combine(client.Home, "overlay-windows.ps1"))) throw new IOException("Update the " + client.Name + " companion to use desktop overlay windows.");
            using (var process = Process.Start(OverlayWindowsCommand(client))) { }
        }
        public Process Update() {
            // The updater takes the same cross-process gate before installation.
            var info = PowerShell(Path.Combine(Root, "Updater", "Update.ps1"), " -NoPause");
            info.CreateNoWindow = Portable; info.WindowStyle = Portable ? ProcessWindowStyle.Hidden : ProcessWindowStyle.Minimized;
            return Process.Start(info);
        }
        public string UpdateMessage(bool busy) {
            if (busy) {
                try {
                    var status = Files.Json(Path.Combine(Updates, "status.json"));
                    return Files.Text(status, "message");
                } catch { return "Updater or engine controls are busy."; }
            }
            if (Portable) return "GitHub releases · Update here, then /reload in WoW when ready.";
            try {
                var source = Files.Json(Path.Combine(Updates, "source.json"));
                if (Files.Text(source, "source") == "github") return "GitHub releases: " + Files.Text(source, "repository");
            } catch (IOException) { } catch (ArgumentException) { } catch (UnauthorizedAccessException) { }
            try {
                var latest = Files.Json(Path.Combine(Updates, "latest.json"));
                string release = Files.Text(latest, "release");
                var last = Files.Json(Path.Combine(Updates, "last-run.json"));
                var results = last["clients"] as Dictionary<string, object>;
                bool current = Files.Text(last, "release") == release && results != null && Clients.All(c => {
                    object entry; if (!results.TryGetValue(c.Key, out entry)) return false;
                    string status = Files.Text(entry as Dictionary<string, object>, "status");
                    return status == "current" || status == "updated";
                });
                return current ? "Latest prepared release was checked for both clients." : "A prepared local release is available to check.";
            } catch { return "Prepare a local release before checking for updates."; }
        }
        public void Log(string value) {
            try {
                string folder = Path.Combine(Root, "Desktop", "data"); Directory.CreateDirectory(folder);
                File.AppendAllText(Path.Combine(folder, "desktop.log"), DateTime.Now.ToString("s") + " " + value + Environment.NewLine);
            } catch { }
        }
        public void PanelStatus(bool trayVisible, bool windowVisible, Dictionary<string, EngineState> states, bool busy) {
            string folder = Path.Combine(Root, "Desktop", "data"); Directory.CreateDirectory(folder);
            var status = new Dictionary<string, object> {
                { "pid", Process.GetCurrentProcess().Id }, { "version", "0.1.0" }, { "at", DateTime.UtcNow.ToString("o") },
                { "trayVisible", trayVisible }, { "windowVisible", windowVisible }, { "controlsBusy", busy },
                { "retailPid", states["retail"].Pid }, { "foreverPid", states["forever"].Pid }
            };
            string target = Path.Combine(folder, "panel-status.json"), temp = target + ".tmp";
            File.WriteAllText(temp, new JavaScriptSerializer().Serialize(status));
            if (File.Exists(target)) File.Replace(temp, target, null); else File.Move(temp, target);
        }
    }
}
