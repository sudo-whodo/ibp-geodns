package powerdns

import (
	"encoding/json"
	"fmt"
	"html"
	"ibp-geodns/config"
	"ibp-geodns/matrixbot"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

var previousStatus = make(map[string]map[string]map[string]bool)
var mu sync.RWMutex

func updateMemberStatus() {
	for result := range resultsChannel {
		go launchUpdate(result)
	}
}

func launchUpdate(result string) {
	results := strings.Split(result, "\n")
	for _, res := range results {
		if strings.TrimSpace(res) == "" {
			continue
		}

		// Print each individual result string
		//log.Printf("Processing result: %s", res)

		var siteStatus config.SiteResults
		var endpointStatus config.EndpointResults

		// Attempt to unmarshal as SiteResults
		if err := json.Unmarshal([]byte(res), &siteStatus); err == nil && siteStatus.ResultType == "site" {
			//log.Printf("Unmarshaled as SiteResults: %+v", siteStatus)
			go updateSiteStatus(siteStatus)
		} else if err := json.Unmarshal([]byte(res), &endpointStatus); err == nil && endpointStatus.ResultType == "endpoint" {
			// Unmarshal as EndpointResults
			//log.Printf("Unmarshaled as EndpointResults: %+v", endpointStatus)
			go updateEndpointStatus(endpointStatus)
		} else {
			// Log the error if unmarshaling fails
			log.Printf("Error parsing result: %v. Result data: %s", err, res)
		}
	}
}

func updateSiteStatus(status config.SiteResults) {
	// log.Printf("Updating site status: %+v", status)
	for memberName, checks := range status.Members {
		for checkName, result := range checks {
			member, memberExists := getMember(memberName)
			if !memberExists {
				continue
			}

			if previousStatus["site"] == nil {
				previousStatus["site"] = make(map[string]map[string]bool)
			}

			if previousStatus["site"][memberName] == nil {
				previousStatus["site"][memberName] = make(map[string]bool)
			}

			if previousStatus["site"][memberName][checkName] != result.Success {
				// log.Printf("Status change detected for site member %s check %s: %v -> %v", memberName, checkName, previousStatus["site"][memberName][checkName], result.Success)

				if result.Success {
					if member.Results[checkName].OfflineTS.IsZero() {
						updateMember("", memberName, checkName, Result{Success: true})
						previousStatus["site"][memberName][checkName] = result.Success
					} else if time.Since(member.Results[checkName].OfflineTS).Seconds() <= float64(configData.MinimumOfflineTime) {
						continue
					}

					if !member.Results[checkName].OfflineTS.IsZero() && time.Since(member.Results[checkName].OfflineTS).Seconds() >= float64(configData.MinimumOfflineTime) {
						updateMember("", memberName, checkName, Result{Success: true})
						previousStatus["site"][memberName][checkName] = result.Success

						if !member.Override {
							sendMatrixMessage(fmt.Sprintf("<b>Adding member</b> <i>%s</i> <b>to all rotations</b><br><i><b>Server:</b> %s</i><br><i><b>Check %s:</b> false -> true</i><BR><b>Result Data:</b> %v", memberName, configData.ServerName, checkName, result.CheckData))
							logStatusChange("Site Status Change", memberName, checkName, false, true, result.CheckData)
						}
					}
				} else {
					updateMember("", memberName, checkName, Result{Success: false, Data: result.CheckError, OfflineTS: time.Now()})

					previousStatus["site"][memberName][checkName] = result.Success
					if !member.Override {
						sendMatrixMessage(fmt.Sprintf("<b>Removing member</b> <i>%s</i> <b>from all rotations</b><br><i><b>Server:</b> %s</i><br><i><b>Check %s:</b> true -> false</i><BR><b>Result Data:</b> %v", memberName, configData.ServerName, checkName, result.CheckData))
						logStatusChange("Site Status Change", memberName, checkName, true, false, result.CheckData)
					}
				}
			} else {
				if !result.Success {
					updateMember("", memberName, checkName, Result{Success: false, Data: result.CheckError, OfflineTS: time.Now()})
				}
			}
		}
	}
}

func updateEndpointStatus(status config.EndpointResults) {
	//log.Printf("Updating endpoint status: %+v", status)
	for endpointURL, members := range status.Endpoint {
		for memberName, checks := range members {
			for checkName, result := range checks {
				compositeKey := fmt.Sprintf("%s::%s", endpointURL, checkName)

				if previousStatus["endpoint"] == nil {
					previousStatus["endpoint"] = make(map[string]map[string]bool)
				}
				if previousStatus["endpoint"][memberName] == nil {
					previousStatus["endpoint"][memberName] = make(map[string]bool)
				}

				member, memberExists := getMember(memberName)
				if !memberExists {
					continue
				}

				if previousStatus["endpoint"][memberName][compositeKey] != result.Success {
					if result.Success {
						if member.Results[compositeKey].OfflineTS.IsZero() {
							updateMember(endpointURL, memberName, compositeKey, Result{Success: true})
							previousStatus["endpoint"][memberName][compositeKey] = result.Success
						} else if time.Since(member.Results[compositeKey].OfflineTS).Seconds() <= float64(configData.MinimumOfflineTime) {
							continue
						}

						if !member.Results[compositeKey].OfflineTS.IsZero() && time.Since(member.Results[compositeKey].OfflineTS).Seconds() >= float64(configData.MinimumOfflineTime) {
							updateMember(endpointURL, memberName, compositeKey, Result{Success: true})

							if !member.Override {
								sendMatrixMessage(fmt.Sprintf(
									"<b>Adding member</b> <i>%s</i> <b>to endpoint</b> <i>%s</i><br>"+
										"<i><b>Server:</b> %s</i><br>"+
										"<i><b>Check %s:</b> false -> true</i><br>"+
										"<b>Result Data:</b> %v",
									memberName, endpointURL, configData.ServerName, compositeKey, result.CheckError))
								logStatusChange("Endpoint Status Change", memberName, compositeKey, false, true, result.CheckError)
							}
						}

					} else {
						updateMember(endpointURL, memberName, compositeKey, Result{Success: false, Data: result.CheckError, OfflineTS: time.Now()})

						previousStatus["endpoint"][memberName][compositeKey] = result.Success

						if !member.Override {
							sendMatrixMessage(fmt.Sprintf(
								"<b>Removing member</b> <i>%s</i> <b>from endpoint</b> <i>%s</i><br>"+
									"<i><b>Server:</b> %s</i><br>"+
									"<i><b>Check %s:</b> true -> false</i><br>"+
									"<b>Result Data:</b> %v",
								memberName, endpointURL, configData.ServerName, compositeKey, result.CheckError))
							logStatusChange("Endpoint Status Change", memberName, compositeKey, true, false, result.CheckError)
						}
					}
				} else {
					if !result.Success && !member.Results[compositeKey].Success {
						updateMember(endpointURL, memberName, compositeKey, Result{Success: false, Data: result.CheckError, OfflineTS: time.Now()})
					}
				}
			}
		}
	}
}

func updateMember(endpointURL, memberName, key string, result Result) {
	if endpointURL != "" {
		var domain string

		if idx := strings.Index(endpointURL, "/"); idx != -1 {
			domain = endpointURL[:idx]
		} else {
			domain = endpointURL
		}

		for i := range powerDNSConfigs {
			dnsConfig := &powerDNSConfigs[i]
			if dnsConfig.Domain == domain {
				if _, memberExists := dnsConfig.Members[memberName]; memberExists {
					dnsConfig.Members[memberName].Results[key] = result
					//log.Printf("Assigned Success=%v to check '%s' for member '%s' in domain '%s'", result.Success, key, memberName, endpointURL)
				}
			}
		}
	} else {
		for i := range powerDNSConfigs {
			dnsConfig := &powerDNSConfigs[i]
			if _, memberExists := dnsConfig.Members[memberName]; memberExists {
				dnsConfig.Members[memberName].Results[key] = result
				//log.Printf("Assigned Success=%v to check '%s' for member '%s' in domain '%s'", result.Success, key, memberName, endpointURL)
			}
		}
	}
}

func getMember(memberName string) (Member, bool) {
	mu.Lock()
	defer mu.Unlock()
	for i := range powerDNSConfigs {
		if member, exists := powerDNSConfigs[i].Members[memberName]; exists {
			if member.Results == nil {
				member.Results = make(map[string]Result)
				powerDNSConfigs[i].Members[memberName] = member
			}
			return member, true
		}
	}
	return Member{}, false
}

func sendMatrixMessage(message string) {
	if configData.Matrix.Enabled == 1 {
		bot, err := matrixbot.NewMatrixBot(configData.Matrix.HomeServerURL, configData.Matrix.Username, configData.Matrix.Password, configData.Matrix.RoomID)
		if err != nil {
			log.Printf("Error initializing Matrix bot: %v", err)
		} else {
			go bot.SendMessage(message)
		}
	}
}

func logStatusChange(changeType, memberName, checkName string, prevSuccess, newSuccess bool, resultData interface{}) {
	//log.Printf("%s: Server %s - member %s - Check %s: %v -> %v - Result Data: %v", changeType, configData.ServerName, memberName, checkName, prevSuccess, newSuccess, resultData)
}

func statusOutput(w http.ResponseWriter, r *http.Request) {
	// Sort the powerDNSConfigs based on the domain name
	sort.SliceStable(powerDNSConfigs, func(i, j int) bool {
		return powerDNSConfigs[i].Domain < powerDNSConfigs[j].Domain
	})

	// Sort members within each powerDNSConfig
	for i := range powerDNSConfigs {
		memberNames := make([]string, 0, len(powerDNSConfigs[i].Members))
		for memberName := range powerDNSConfigs[i].Members {
			memberNames = append(memberNames, memberName)
		}
		sort.Strings(memberNames)

		sortedMembers := make(map[string]Member)
		for _, memberName := range memberNames {
			sortedMembers[memberName] = powerDNSConfigs[i].Members[memberName]
		}
		mu.Lock()
		powerDNSConfigs[i].Members = sortedMembers
		mu.Unlock()
	}

	// Collect all unique members
	uniqueMembersMap := make(map[string]struct{})
	for _, config := range powerDNSConfigs {
		for memberName := range config.Members {
			uniqueMembersMap[memberName] = struct{}{}
		}
	}
	uniqueMembers := make([]string, 0, len(uniqueMembersMap))
	for memberName := range uniqueMembersMap {
		uniqueMembers = append(uniqueMembers, memberName)
	}
	sort.Strings(uniqueMembers)

	var sb strings.Builder

	// Start HTML document with enhanced styles
	sb.WriteString(fmt.Sprintf("<!DOCTYPE html><html><head><meta charset='UTF-8'><title>%s Status Page</title>", htmlEscape(configData.ServerName)))
	sb.WriteString(`<style>
		body { 
			font-family: Arial, sans-serif; 
			font-size: 14px; 
			background-color: #f4f4f4; 
			margin: 0; 
			padding: 20px;
		}
		h1 { 
			text-align: center; 
			color: #333; 
			margin-bottom: 30px;
		}
		/* Dropdown Styles */
		#member-filter {
			margin-bottom: 20px;
			padding: 8px 12px;
			font-size: 14px;
			border-radius: 4px;
			border: 1px solid #ccc;
		}
		.domain-header {
			background-color: #2ecc71; /* Green by default */
			color: white;
			padding: 8px 12px;
			margin-bottom: 5px;
			cursor: pointer;
			border-radius: 4px;
			font-size: 16px;
			display: flex;
			align-items: center;
			gap: 10px;
			transition: background-color 0.3s ease;
			position: relative;
		}
		.domain-header.failed {
			background-color: #e74c3c; /* Red if any member failed */
		}
		.domain-header .arrow {
			margin-left: auto;
			transition: transform 0.3s ease;
			font-size: 16px;
		}
		.domain-header.active .arrow {
			transform: rotate(90deg);
		}
		.domain-header .info {
			font-size: 12px;
			opacity: 0.8;
		}
		.domain-content {
			display: none;
			margin-bottom: 20px;
			border: 1px solid #ddd;
			border-radius: 4px;
			background-color: white;
			padding: 10px;
		}
		table {
			width: 100%;
			border-collapse: collapse;
			margin-top: 10px;
			table-layout: fixed;
		}
		th, td {
			border: 1px solid #ddd;
			padding: 6px;
			text-align: left;
			vertical-align: top;
			word-wrap: break-word;
		}
		th.member, td.member {
			width: 12.5%; /* Reduced by 50% from original ~25% */
		}
		th.override, td.override {
			width: 10%; /* Reduced by 50% from original ~20% */
		}
		th.ipv4, td.ipv4,
		th.ipv6, td.ipv6 {
			width: 12%; /* Reduced by 33% from original ~18% */
		}
		th.lat, td.lat,
		th.lon, td.lon {
			width: 5%; /* Reduced by 75% from original ~20% */
		}
		th.site-results, td.site-results,
		th.endpoint-results, td.endpoint-results {
			width: 22.5%; /* Allocated remaining space */
		}
		th {
			background-color: #f2f2f2;
			color: #333;
			font-size: 14px;
		}
		td {
			font-size: 13px;
		}
		.result-success {
			color: green;
			font-weight: bold;
		}
		.result-failure {
			color: red;
			font-weight: bold;
		}
		ul {
			margin: 0;
			padding-left: 20px;
			list-style-type: none;
		}
		li {
			margin: 0;
			padding: 0;
		}
		@media screen and (max-width: 768px) {
			body {
				padding: 10px;
			}
			h1 {
				font-size: 24px;
			}
			#member-filter {
				width: 100%;
			}
			.domain-header {
				font-size: 14px;
			}
			th, td {
				font-size: 12px;
			}
			th.member, td.member,
			th.override, td.override,
			th.ipv4, td.ipv4,
			th.ipv6, td.ipv6,
			th.lat, td.lat,
			th.lon, td.lon,
			th.site-results, td.site-results,
			th.endpoint-results, td.endpoint-results {
				width: auto;
			}
		}
	</style></head><body>`)

	// Server Title
	sb.WriteString(fmt.Sprintf("<h1>%s Status Page</h1>", htmlEscape(configData.ServerName)))

	// Render the dropdown for member filtering
	sb.WriteString(`<select id='member-filter'>`)
	sb.WriteString(`<option value='all'>All Members</option>`)
	for _, member := range uniqueMembers {
		// Set the option value to the raw member name and display the escaped member name
		sb.WriteString(fmt.Sprintf("<option value='%s'>%s</option>", htmlEscape(member), htmlEscape(member)))
	}
	sb.WriteString(`</select>`)

	// Iterate over each domain
	for _, config := range powerDNSConfigs {
		totalMembers := len(config.Members)
		onlineMembers := 0
		offlineMembers := 0
		domainFailed := false

		// Map to store each member's status within this domain
		memberStatuses := make(map[string]string)

		for _, member := range config.Members {
			memberOnline := true
			for _, result := range member.Results {
				if !result.Success || !result.OfflineTS.IsZero() {
					memberOnline = false
					break
				}
			}
			if memberOnline {
				onlineMembers++
				memberStatuses[member.MemberName] = "success"
			} else {
				offlineMembers++
				memberStatuses[member.MemberName] = "failure"
				domainFailed = true
			}
		}

		// Marshal memberStatuses to JSON
		memberStatusesJSON, err := json.Marshal(memberStatuses)
		if err != nil {
			log.Printf("Error marshalling member statuses for domain %s: %v", config.Domain, err)
			// In case of error, set to empty JSON object
			memberStatusesJSON = []byte("{}")
		}

		// Assign class based on domain status (aggregate of all members)
		domainClass := "domain-header"
		if domainFailed {
			domainClass += " failed"
		}

		// Domain header with member counts and arrow
		sb.WriteString(fmt.Sprintf("<div class='%s' data-domain='%s' data-members='%s' onclick=\"toggleDomain('%s')\">",
			domainClass,
			htmlEscape(config.Domain),
			htmlEscape(string(memberStatusesJSON)),
			htmlEscape(config.Domain),
		))
		sb.WriteString(fmt.Sprintf("Domain: %s", htmlEscape(config.Domain)))
		sb.WriteString(fmt.Sprintf("<span class='info'>Members: Total: %d, Offline: %d, Online: %d</span>", totalMembers, offlineMembers, onlineMembers))
		sb.WriteString("<span class='arrow'>&#9654;</span>")
		sb.WriteString("</div>")

		// Domain content div
		sb.WriteString(fmt.Sprintf("<div class='domain-content' id='%s-content'>", htmlEscape(config.Domain)))
		sb.WriteString("<table>")
		sb.WriteString("<tr><th class='member'>Member</th><th class='override'>Override</th><th class='ipv4'>IPv4</th><th class='ipv6'>IPv6</th><th class='lat'>Lat</th><th class='lon'>Lon</th><th class='site-results'>Site Results</th><th class='endpoint-results'>Endpoint Results</th></tr>")

		// Get sorted member names
		memberNames := sortedMemberNames(config.Members)

		for _, memberName := range memberNames {
			member := config.Members[memberName]
			sb.WriteString("<tr>")
			sb.WriteString(fmt.Sprintf(
				"<td class='member'>%s</td>"+
					"<td class='override'>%t</td>"+
					"<td class='ipv4'>%s</td>"+
					"<td class='ipv6'>%s</td>"+
					"<td class='lat'>%.2f</td>"+
					"<td class='lon'>%.2f</td>",
				htmlEscape(memberName),
				member.Override,
				htmlEscape(member.IPv4),
				htmlEscape(member.IPv6),
				member.Latitude,
				member.Longitude,
			))

			// Separate site and endpoint results
			siteResults, endpointResults := separateResults(config.Domain, member.Results)

			// Display Site Results
			sb.WriteString("<td class='site-results'><ul>")
			for checkName, result := range siteResults {
				sb.WriteString("<li>")
				if result.Success {
					sb.WriteString(fmt.Sprintf("<span class='result-success'>%s: %v</span>", htmlEscape(checkName), result.Success))
				} else {
					sb.WriteString(fmt.Sprintf("<span class='result-failure'>%s: %v (%s)</span>", htmlEscape(checkName), result.Success, htmlEscape(result.Data)))
				}
				if !result.OfflineTS.IsZero() {
					sb.WriteString(fmt.Sprintf(", %s", result.OfflineTS.Format("2006-01-02 15:04")))
				}
				sb.WriteString("</li>")
			}
			sb.WriteString("</ul></td>")

			// Display Endpoint Results
			sb.WriteString("<td class='endpoint-results'><ul>")
			for checkName, result := range endpointResults {
				sb.WriteString("<li>")
				if result.Success {
					sb.WriteString(fmt.Sprintf("<span class='result-success'>%s: %v</span>", htmlEscape(checkName), result.Success))
				} else {
					sb.WriteString(fmt.Sprintf("<span class='result-failure'>%s: %v (%s)</span>", htmlEscape(checkName), result.Success, htmlEscape(result.Data)))
				}
				if !result.OfflineTS.IsZero() {
					sb.WriteString(fmt.Sprintf(", %s", result.OfflineTS.Format("2006-01-02 15:04")))
				}
				sb.WriteString("</li>")
			}
			sb.WriteString("</ul></td>")

			sb.WriteString("</tr>")
		}

		sb.WriteString("</table>")
		sb.WriteString("</div>") // Close domain-content
	}

	// JavaScript for interactivity and filtering
	sb.WriteString(`
	<script>
		// Toggle domain content visibility
		function toggleDomain(domain) {
			var content = document.getElementById(domain + '-content');
			var header = document.querySelector(".domain-header[data-domain='" + domain + "']");
			if (content.style.display === "block") {
				content.style.display = "none";
				header.classList.remove("active");
			} else {
				content.style.display = "block";
				header.classList.add("active");
			}
		}

		// Handle member filtering
		document.getElementById('member-filter').addEventListener('change', function() {
			var selectedMember = this.value;
			var domains = document.getElementsByClassName('domain-header');

			for (var i = 0; i < domains.length; i++) {
				var domain = domains[i];
				var domainName = domain.getAttribute('data-domain');
				var membersStatus = domain.getAttribute('data-members');
				var content = document.getElementById(domainName + '-content');
				var rows = content.getElementsByTagName('tr');
				var hasSelectedMember = false;

				// Iterate over table rows to find if the selected member exists
				for (var j = 1; j < rows.length; j++) { // Start from 1 to skip header row
					var memberCell = rows[j].getElementsByClassName('member')[0];
					var memberName = memberCell.textContent.trim();

					if (selectedMember === 'all' || memberName === selectedMember) {
						if (selectedMember === 'all') {
							rows[j].style.display = '';
						} else {
							rows[j].style.display = '';
							hasSelectedMember = true;
						}
					} else {
						rows[j].style.display = 'none';
					}
				}

				// Show or hide the domain based on whether it has the selected member
				if (selectedMember === 'all') {
					domain.style.display = '';
					content.style.display = 'none';
					domain.classList.remove('active');
				} else if (hasSelectedMember) {
					domain.style.display = '';
					content.style.display = 'block';
					domain.classList.add('active');

					// Parse membersStatus JSON
					var membersStatusObj = {};
					try {
						membersStatusObj = JSON.parse(membersStatus);
					} catch (e) {
						console.error("Error parsing membersStatus for domain: " + domainName, e);
					}

					// Get the status of the selected member
					var memberStatus = membersStatusObj[selectedMember];

					// Update the domain header's class based on memberStatus
					if (memberStatus === "failure") {
						domain.classList.add("failed");
					} else {
						domain.classList.remove("failed");
					}
				} else {
					domain.style.display = 'none';
					content.style.display = 'none';
					domain.classList.remove('active');
				}
			}
		});
	</script>
	`)

	// Close HTML document
	sb.WriteString("</body></html>")

	// Write the response
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(sb.String()))
	if err != nil {
		log.Printf("Error writing response: %v", err)
	}
}

// Helper function to sort and return member names
func sortedMemberNames(members map[string]Member) []string {
	memberNames := make([]string, 0, len(members))
	for name := range members {
		memberNames = append(memberNames, name)
	}
	sort.Strings(memberNames)
	return memberNames
}

// Helper function to separate site and endpoint results
func separateResults(domain string, results map[string]Result) (map[string]Result, map[string]Result) {
	siteResults := make(map[string]Result)
	endpointResults := make(map[string]Result)

	for key, result := range results {
		if strings.Contains(key, "::") {
			// Endpoint result
			parts := strings.SplitN(key, "::", 2)
			endpointURL := parts[0]
			checkName := parts[1]

			var path string

			if idx := strings.Index(endpointURL, "/"); idx != -1 {
				temp := strings.SplitN(endpointURL, "/", 2)
				endpointURL = temp[0]
				path = temp[1]
			}

			if endpointURL != domain {
				continue
			}

			var displayName string
			if path != "" {
				displayName = fmt.Sprintf("%s Path: %s", checkName, path)
			} else {
				displayName = checkName
			}

			endpointResults[displayName] = result
		} else {
			// Site result
			siteResults[key] = result
		}
	}

	return siteResults, endpointResults
}

// Helper function to escape HTML content
func htmlEscape(s string) string {
	return html.EscapeString(s)
}
