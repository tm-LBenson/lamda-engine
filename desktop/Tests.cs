using System;
using System.Collections.Generic;
using System.Drawing;
using System.Drawing.Imaging;
using System.IO;
using System.Threading.Tasks;
using System.Windows.Forms;

namespace LamdaUI {
    internal static class DesktopTests {
        private static int count;
        private static void Check(bool condition, string message) { count++; if (!condition) throw new Exception(message); }
        private static Dictionary<string, object> State(int pid, string client) {
            return new Dictionary<string, object> { { "pid", pid }, { "client", client }, { "inputStatus", "idle" } };
        }
        public static int Run(string output) {
            var now = DateTime.UtcNow;
            var notices = new SimNotifications();
            var simJob = new Dictionary<string, object> { {"id", "job1"}, {"requestID", "request1"}, {"task", "compare"}, {"state", "running"} };
            var simStatus = new Dictionary<string, object> { {"schema", 1}, {"client", "retail"}, {"jobs", new object[] {simJob}} };
            Check(notices.Observe(simStatus, "retail", now) == null, "running sim has no completion notice");
            simJob["state"] = "complete"; simJob["finishedAt"] = now.ToString("o");
            Check(notices.Observe(simStatus, "forever", now) == null, "sim notice client isolation");
            Check(notices.Observe(simStatus, "retail", now).Contains("/reload"), "completed sim explains result import");
            Check(notices.Observe(simStatus, "retail", now) == null, "sim completion notice not repeated");
            simJob["id"] = "old"; simJob["finishedAt"] = now.AddHours(-1).ToString("o");
            Check(notices.Observe(simStatus, "retail", now) == null, "old saved runs do not generate notifications");
            simJob["id"] = "failed"; simJob["finishedAt"] = now.ToString("o"); simJob["state"] = "failed";
            Check(notices.Observe(simStatus, "retail", now).Contains("reason"), "failed sim explains where to find error");
            simStatus["jobs"] = new object[] {
                new Dictionary<string, object> {{"id", "new-complete"}, {"requestID", "r2"}, {"task", "compare"}, {"state", "complete"}, {"finishedAt", now.ToString("o")}},
                new Dictionary<string, object> {{"id", "earlier-failed"}, {"requestID", "r1"}, {"task", "compare"}, {"state", "failed"}, {"finishedAt", now.AddSeconds(-30).ToString("o")}}
            };
            Check(notices.Observe(simStatus, "retail", now).StartsWith("Sim finished."), "latest completion wins when several statuses arrive together");
            Check(EngineState.Classify(true, false, 123, false, "1", State(123, "retail"), now, now, "retail").Healthy, "matching recent process");
            Check(!EngineState.Classify(true, false, 123, true, "1", State(456, "retail"), now, now, "retail").Healthy, "stale process id");
            Check(!EngineState.Classify(true, false, 123, true, "1", State(123, "forever"), now, now, "retail").Healthy, "wrong client state");
            Check(!EngineState.Classify(true, false, 123, false, "1", State(123, "retail"), now.AddSeconds(-20), now, "retail").Healthy, "stale heartbeat");
            Check(!EngineState.Classify(true, false, 123, false, "1", State(123, "retail"), now.AddSeconds(10), now, "retail").Healthy, "future heartbeat");
            Check(!EngineState.Classify(true, false, 0, true, "1", State(123, "retail"), now, now, "retail").Running, "leftover state cannot mean running");
            Check(EngineState.Classify(true, true, 0, false, "1", null, now, now, "retail").Unknown, "inaccessible process is not stopped");
            Check(Files.Same(@"D:\LamdaUI\Retail\lamda-engine.exe", @"d:\lamdaui\retail\lamda-engine.exe"), "Windows path case");
            Check(!Files.Same(@"D:\LamdaUI\Retail\lamda-engine.exe", @"D:\LamdaUI\Forever\lamda-engine.exe"), "client process isolation");
            Check(!Files.Same(@"D:\LamdaUI\Retail\lamda-engine.exe", @"D:\LamdaUI\Retail-old\lamda-engine.exe"), "no path prefix ownership");
            Check(new Client(@"D:\LamdaUI", "forever", @"E:\Games\_forever_").GameExecutable == "Wow.exe", "Forever production executable");
            Check(new Client(@"D:\LamdaUI", "forever", @"E:\Games\_classic_beta_").GameExecutable == "WowB.exe", "Forever legacy beta executable");
            Check(new Client(@"D:\LamdaUI", "retail", @"E:\Games\_retail_").GameExecutable == "Wow.exe", "Retail executable unchanged");
            Check(new Client(@"D:\LamdaUI", "forever", "").GameExecutable == "WowB.exe", "unconfigured Forever path remains readable");
            Check(Files.Quote(@"D:\World of Warcraft\a") == "\"D:\\World of Warcraft\\a\"", "space quoting");
            var overlayRetail = Controller.OverlayWindowsCommand(new Client(@"D:\LamdaUI", "retail", ""));
            var overlayForever = Controller.OverlayWindowsCommand(new Client(@"D:\LamdaUI", "forever", ""));
            Check(overlayRetail.Arguments.Contains("-ManageWindows") && overlayRetail.Arguments.Contains(@"D:\LamdaUI\Retail\data\frame-display.json"), "Retail overlay windows scope");
            Check(overlayForever.Arguments.Contains(@"D:\LamdaUI\Forever\data\frame-display.json") && !overlayForever.Arguments.Contains(@"D:\LamdaUI\Retail\"), "Forever overlay windows scope");
            bool rejected = false;
            try { Files.Quote("invalid\"path"); } catch (ArgumentException) { rejected = true; }
            Check(rejected, "quote injection rejected");
            string testGate = Gate.Name + ".Tests." + Guid.NewGuid();
            using (var gate = new Gate(testGate)) {
                Check(gate.Acquired, "control gate initially free");
                Check(Task.Run(() => Gate.Busy(testGate)).Result, "different control thread excluded");
            }
            Check(!Gate.Busy(testGate), "gate released after operation");
            string fixture = Path.Combine(Path.GetDirectoryName(output), "portable-test-" + Guid.NewGuid().ToString("N"));
            try {
                Directory.CreateDirectory(Path.Combine(fixture, "Updater"));
                File.WriteAllText(Path.Combine(fixture, "Updater", "updater.json"), new System.Web.Script.Serialization.JavaScriptSerializer().Serialize(
                    new Dictionary<string, object> {{"product", "LamdaUI"}, {"schema", 2}, {"clients", new Dictionary<string, object> {{"retail", @"E:\Games\WoW\_retail_"}}}}));
                var portable = new Controller(fixture);
                Check(portable.Portable, "standalone configuration needs no WSL project");
                Check(portable.Updates == Path.Combine(fixture, "Updater", "data"), "standalone update status stays with installation");
                Check(portable.Clients[0].Game == @"E:\Games\WoW\_retail_", "configured game path replaces development machine path");
                Check(portable.Clients[1].Game == "", "unselected game remains unconfigured");
                Check(portable.UpdateMessage(false).Contains("GitHub"), "standalone update guidance");
            } finally { if (Directory.Exists(fixture)) Directory.Delete(fixture, true); }
            var clients = new [] { new Client(@"D:\LamdaUI", "retail", ""), new Client(@"D:\LamdaUI", "forever", "") };
            using (var icon = Icon.ExtractAssociatedIcon(Application.ExecutablePath))
            using (var form = new Dashboard(clients, icon, (c, a) => { }, c => { }, c => { }, () => { }, () => { })) {
                var live = EngineState.Classify(true, false, 123, false, "0.7.13-retail.2", State(123, "retail"), now, now, "retail");
                var card = form.Cards["retail"];
                card.Bind(live, false);
                Check(!card.Start.Enabled && card.Stop.Enabled && card.Restart.Enabled, "running engine controls");
                card.Bind(live, true);
                Check(!card.Start.Enabled && !card.Stop.Enabled && !card.Restart.Enabled, "update blocks all engine controls");
                card.Bind(EngineState.Classify(true, false, 0, false, "1", null, now, now, "retail"), false);
                Check(card.Start.Enabled && !card.Stop.Enabled && !card.Restart.Enabled, "stopped engine controls");
                card.Bind(EngineState.Classify(true, true, 0, false, "1", null, now, now, "retail"), false);
                Check(!card.Start.Enabled && !card.Stop.Enabled && !card.Restart.Enabled, "unknown process controls disabled");
                CheckBounds(form);
                form.Quitting = true;
            }
            File.WriteAllText(output, "PASS: " + count + " desktop checks; heartbeat identity/age, client path isolation, quoting, shared control gate and UI controls." + Environment.NewLine);
            return 0;
        }
        private static void CheckBounds(Control parent) {
            foreach (Control child in parent.Controls) {
                Check(child.Left >= 0 && child.Top >= 0 && child.Right <= parent.ClientSize.Width && child.Bottom <= parent.ClientSize.Height,
                      "control clipped: " + child.Text);
                CheckBounds(child);
            }
        }
        public static int Render(string output) {
            var clients = new [] { new Client(@"D:\LamdaUI", "retail", ""), new Client(@"D:\LamdaUI", "forever", "") };
            using (var icon = Icon.ExtractAssociatedIcon(Application.ExecutablePath))
            using (var form = new Dashboard(clients, icon, (c, a) => { }, c => { }, c => { }, () => { }, () => { })) {
                var now = DateTime.UtcNow;
                var retail = EngineState.Classify(true, false, 123, true, "0.7.13-retail.2", State(123, "retail"), now, now, "retail");
                retail.AddonVersion = "0.45.5-retail.1";
                var forever = EngineState.Classify(true, false, 0, false, "0.7.13-forever.3", null, now, now, "forever");
                forever.AddonVersion = "0.45.5-forever.1";
                form.Cards["retail"].Bind(retail, false); form.Cards["forever"].Bind(forever, false);
                form.UpdateStatus.Text = "Latest prepared release was applied to both clients.";
                form.RenderOnly = true; form.ShowInTaskbar = false; form.StartPosition = FormStartPosition.Manual;
                form.Location = new Point(-10000, -10000); form.Show(); Application.DoEvents();
                using (var bitmap = new Bitmap(form.Width, form.Height)) {
                    form.DrawToBitmap(bitmap, new Rectangle(Point.Empty, bitmap.Size)); bitmap.Save(output, ImageFormat.Png);
                }
                form.Close(); Check(!form.IsDisposed && !form.Visible, "window close hides to tray");
                form.Quitting = true; form.Close();
            }
            return 0;
        }
    }
}
