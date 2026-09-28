"""Regenerate the checked-in synthetic paired replay transcript. No account data."""
import json
from pathlib import Path

# operation, method, escaped path, decrypted query, decrypted body, synthetic result
device = {"id": "device-1", "name": "Desk lamp", "category": "dj"}
user = {"user_id": "user-2", "nick_name": "Guest"}
pairs = [
    ("getDeviceDetails", "GET", "/v1.0/devices/device-1", None, None, device),
    ("getDevicesByUser", "GET", "/v1.0/users/user-1/devices", None, None, [device]),
    ("updateDeviceFunctionName", "PUT", "/v1.0/devices/device-1/functions/switch", None, {"name": "Power"}, True),
    ("getDeviceLogs", "GET", "/v1.0/devices/device-1/logs", {"type":"report", "start_time":0, "end_time":0}, None, {"logs":[], "has_next":False}),
    ("resetDeviceFactory", "PUT", "/v1.0/devices/device-1/reset-factory", None, None, True),
    ("getSubDevices", "GET", "/v1.0/devices/device-1/sub-devices", None, None, []),
    ("getFactoryInfos", "GET", "/v1.0/devices/factory-infos", {"device_ids":"device-1"}, None, []),
    ("addDeviceUser", "POST", "/v1.0/devices/device-1/user", None, {"nick_name":"Guest", "sex":0}, "user-2"),
    ("updateDeviceUser", "PUT", "/v1.0/devices/device-1/users/user-2", None, {"nick_name":"Guest", "sex":0}, True),
    ("getDeviceUser", "GET", "/v1.0/devices/device-1/users/user-2", None, None, user),
    ("listDeviceUsers", "GET", "/v1.0/devices/device-1/users", None, None, [user]),
    ("deleteDeviceUser", "DELETE", "/v1.0/devices/device-1/users/user-2", None, None, True),
    ("updateMultiOutletName", "PUT", "/v1.0/devices/device-1/multiple-name", None, {"identifier":"outlet-1", "name":"Desk"}, True),
    ("listMultiOutletNames", "GET", "/v1.0/devices/device-1/multiple-names", None, None, [{"identifier":"outlet-1", "name":"Desk"}]),
    ("updateDeviceName", "PUT", "/v1.0/devices/device-1", None, {"name":"Desk lamp"}, True),
    ("sendDeviceCommands", "POST", "/v1.1/m/thing/device-1/commands", None, {"commands":[{"code":"switch", "value":True}]}, True),
    ("queryDeviceSpecification", "GET", "/v1.1/m/life/device-1/specifications", None, None, {}),
    ("refreshAccessToken", "GET", "/v1.0/m/token/synthetic-refresh-token", None, None, {"expireTime":7200,"uid":"synthetic-user-id","accessToken":"synthetic-rotated-access","refreshToken":"synthetic-rotated-refresh"}),
    ("getMessageQueueConfig", "POST", "/v1.0/m/life/ha/access/config", None, {"linkId":"<uuid>"}, {"url":"ssl://mqtt.example.invalid:8883","clientId":"synthetic-mqtt-client","username":"synthetic-user","password":"synthetic-password","expireTime":7200,"topic":{"ownerId":{"sub":"cloud/owner/in/channel"},"devId":{"sub":"cloud/device/{devId}/in/channel"}}}),
    ("startRTCSession", "POST", "/v1.0/m/life/ipc/device-1/webrtc/session", None, {"sdp":"v=0 synthetic offer","type":"offer"}, {"session_id":"session-1","sdp":"v=0 synthetic answer"}),
    ("stopRTCSession", "DELETE", "/v1.0/m/life/ipc/device-1/webrtc/session/session-1", None, None, True),
    ("deleteDevice", "DELETE", "/v1.0/devices/device-1", None, None, True),
]
output=[]
for operation,method,path,query,body,result in pairs:
    request={"method":method,"origin":"https://api.example.invalid","path":path,
             "query":{"encdata":["<encrypted>"]} if query is not None else {},
             "headers":{"X-appKey":["synthetic-client-id"],"X-requestId":["<uuid>"],
                        "X-sign":["<tuya-signature>"],"X-time":["<unix-millis>"],
                        "X-token":["synthetic-access-token"]},
             "body":"<encrypted-json>" if body is not None else ""}
    if query is not None: request["plain_query"]=query
    if body is not None: request["plain_body"]=body
    entry={"operation_id":operation,"request":request,
                   "response":{"status":200,"headers":{"Content-Type":["application/json"]},
                               "body":{"success":True,"t":1700000000000,"result":result}}}
    if operation=="addDeviceUser": entry["response_transform"]="encrypted-string-result"
    output.append(entry)
Path(__file__).with_name("remaining-operations.synthetic.json").write_text(json.dumps(output,indent=2)+"\n",encoding="utf-8")
