using System;
using System.Collections;
using System.Collections.Generic;
using System.Globalization;

namespace LamdaUI {
    // No WoW input, clipboard commands, browser launch, or automatic UI reload.
    internal sealed class SimNotifications {
        private readonly Dictionary<string, string> seen = new Dictionary<string, string>();
        public string Observe(Dictionary<string, object> status, string client, DateTime now) {
            if (status == null || Files.Text(status, "schema") != "1" || Files.Text(status, "client") != client) return null;
            object raw;
            if (!status.TryGetValue("jobs", out raw)) return null;
            var jobs = raw as IEnumerable;
            if (jobs == null) return null;
            string message = null;
            DateTime newest = DateTime.MinValue;
            foreach (object entry in jobs) {
                var job = entry as Dictionary<string, object>;
                if (job == null) continue;
                string id = Files.Text(job, "id"), state = Files.Text(job, "state"), previous;
                if (String.IsNullOrEmpty(id) || String.IsNullOrEmpty(Files.Text(job, "requestID"))) continue;
                string key = client + ":" + id;
                seen.TryGetValue(key, out previous); seen[key] = state;
                DateTime finished;
                bool recent = DateTime.TryParse(Files.Text(job, "finishedAt"), CultureInfo.InvariantCulture,
                    DateTimeStyles.AdjustToUniversal | DateTimeStyles.AssumeUniversal, out finished) &&
                    finished <= now.AddSeconds(5) && finished >= now.AddMinutes(-2);
                if (state == previous || !recent || finished < newest) continue;
                string candidate = null;
                if (state == "complete" && Files.Text(job, "task") != "cancel")
                    candidate = Files.Text(status, "exportError") == "" ?
                        "Sim finished. In WoW, /reload then /lsim to see your results." :
                        "Sim finished, but the in-game results file could not be updated. Check the engine log.";
                else if (state == "failed" || state == "interrupted")
                    candidate = "Sim could not finish. In WoW, /reload then /lsim to see the reason.";
                else if (state == "cancelled") candidate = "Sim cancelled. Reload WoW's UI to refresh its status.";
                if (candidate != null) { message = candidate; newest = finished; }
            }
            // Only the latest twelve jobs per client are exported; cap session bookkeeping.
            if (seen.Count > 1000) seen.Clear();
            return message;
        }
    }
}
