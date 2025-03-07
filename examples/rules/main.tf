terraform {
  required_providers {
    secops = {
      source = "hashicorp.com/edu/secops"
    }
  }
  required_version = ">= 1.1.0"
}

provider "secops" {
  base_url = "https://eu-chronicle.googleapis.com/v1alpha/projects/826325533751/locations/eu/instances/87b64822-b99b-453e-a4bc-ab380e798f87"
}

resource "secops_reference_list" "tectonics_sa_allowed_impersonations" {
  display_name        = "tectonics_allowed_impersonations"
  description = "aaaa"
  entries = [
    "// Enter values for the list, one value on each line.",
    "// To add a comment, use a double slash followed by the comment. Any text after",
    "// the double slash and any spaces before it are ignored.",
    "// (This line is an example of a comment.)",
    "hws-filter-sync-events-user@tctn-eu-services-979d.iam.gserviceaccount.com -\u003e projects/-/serviceAccounts/hws-filter-task-user@tctn-eu-services-979d.iam.gserviceaccount.com",
    "hws-retriever-deploy@tctn-eu-services-979d.iam.gserviceaccount.com -\u003e projects/-/serviceAccounts/hws-retriever-user@tctn-eu-services-979d.iam.gserviceaccount.com",
    "usercontent-builder@tctn-eu-services-979d.iam.gserviceaccount.com -\u003e projects/-/serviceAccounts/usercontent-user@tctn-eu-services-979d.iam.gserviceaccount.com",
    "device-reporter-deploy@tctn-eu-services-979d.iam.gserviceaccount.com -\u003e projects/-/serviceAccounts/device-reporter-user@tctn-eu-services-979d.iam.gserviceaccount.com",
    "seismograph-deploy@tctn-aus-services-7dac.iam.gserviceaccount.com -\u003e projects/-/serviceAccounts/seismograph-user@tctn-aus-services-7dac.iam.gserviceaccount.com",
    "pdfer-builder@tctn-eu-services-979d.iam.gserviceaccount.com -> projects/-/serviceAccounts/pdfer-user@tctn-eu-services-979d.iam.gserviceaccount.com",
  ]
}
