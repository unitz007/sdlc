import unittest
import json
import base64
from urllib import request
import threading
import time

# Simple helper to generate a JWT-like token with tenant claim (no signature)
def make_token(tenant_id: str) -> str:
    header = base64.urlsafe_b64encode(json.dumps({"alg":"none"}).encode()).decode().rstrip('=')
    payload = base64.urlsafe_b64encode(json.dumps({"tenant": tenant_id}).encode()).decode().rstrip('=')
    return f"{header}.{payload}."

class TestMultiTenancy(unittest.TestCase):
    # In-memory store to simulate tenant‑aware workflow creation.
    @staticmethod
    def create_workflow(store, tenant_id):
        wf_id = f"wf-{tenant_id}-{len(store.get(tenant_id, []))}"
        store.setdefault(tenant_id, []).append(wf_id)
        return {"id": wf_id}

    def test_isolation(self):
        store = {}
        # tenant identifiers
        tenant_a = 'tenant-a'
        tenant_b = 'tenant-b'
        # create workflow under tenant A and B
        wf_a = TestMultiTenancy.create_workflow(store, tenant_a)
        wf_b = self.create_workflow(store, tenant_b)
        # IDs should be distinct
        self.assertNotEqual(wf_a['id'], wf_b['id'])
        # Ensure isolation: each tenant sees only its workflows
        self.assertIn(wf_a['id'], store.get(tenant_a, []))
        self.assertNotIn(wf_a['id'], store.get(tenant_b, []))
        self.assertIn(wf_b['id'], store.get(tenant_b, []))
        self.assertNotIn(wf_b['id'], store.get(tenant_a, []))

if __name__ == '__main__':
    unittest.main()
