import time
import requests

# GitHub Copilot Client ID (VSCode)
CLIENT_ID = "Iv1.b507a08c87ecfe98"

def get_device_code():
    response = requests.post(
        "https://github.com/login/device/code",
        headers={"Accept": "application/json"},
        data={"client_id": CLIENT_ID, "scope": "read:user"}
    )
    response.raise_for_status()
    return response.json()

def get_access_token(device_code):
    while True:
        time.sleep(5.5)
        response = requests.post(
            "https://github.com/login/oauth/access_token",
            headers={"Accept": "application/json"},
            data={
                "client_id": CLIENT_ID,
                "device_code": device_code,
                "grant_type": "urn:ietf:params:oauth:grant-type:device_code"
            }
        )
        if response.status_code == 200:
            data = response.json()
            if "access_token" in data:
                return data["access_token"]
            if data.get("error") != "authorization_pending":
                raise Exception(f"Error getting access token: {data.get('error_description')}")
        else:
            print(f"Waiting for authorization... ({response.status_code})")

def get_copilot_token(access_token):
    response = requests.get(
        "https://api.github.com/copilot_internal/v2/token",
        headers={
            "Authorization": f"token {access_token}",
            "Editor-Version": "vscode/1.85.1",
            "Editor-Plugin-Version": "copilot/1.143.0",
            "User-Agent": "GithubCopilot/1.143.0"
        }
    )
    if response.status_code != 200:
         raise Exception(f"Failed to get Copilot token: {response.text}")
    
    return response.json()["token"]

def main():
    print("Initiating GitHub Device Flow...")
    device_data = get_device_code()
    
    print(f"\nPlease visit: {device_data['verification_uri']}")
    print(f"And enter code: {device_data['user_code']}")
    
    print("\nWaiting for authentication...")
    access_token = get_access_token(device_data['device_code'])
    
    print("\nAuthenticated! Retrieving Copilot token...")
    copilot_token = get_copilot_token(access_token)
    
    print("\nSUCCESS! Here is your Copilot API Key:")
    print(f"\n{copilot_token}\n")
    
    # Optional: Write to .env
    with open(".env", "a") as f:
        f.write(f"\nCOPILOT_API_KEY=\"{copilot_token}\"\n")
    print("Appended COPILOT_API_KEY to .env")

if __name__ == "__main__":
    main()
