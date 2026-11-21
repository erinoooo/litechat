"""End-to-end tests: the real binary against a fake OpenAI-compatible server.

    LITECHAT_BINARY=./litechat PYTHONPATH=python python3 -m unittest discover -s python/tests -v

Standard library only; no model or network needed.
"""

import json
import os
import socket
import tempfile
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import litechat


class FakeOpenAI(BaseHTTPRequestHandler):
    """Streams "<reasoning> Echo: <last message>" for model "echo";
    fails for model "broken"."""

    def log_message(self, *args):
        pass

    def _json(self, code, obj):
        body = json.dumps(obj).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path.endswith("/models"):
            self._json(200, {"data": [{"id": "echo"}, {"id": "broken"}]})
        else:
            self._json(404, {})

    def do_POST(self):
        req = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        if req["model"] == "broken":
            return self._json(500, {"error": {"message": "the model fell over"}})
        last = req["messages"][-1]["content"]
        if isinstance(last, list):
            last = " ".join(p.get("text", "[image]") for p in last)
        if not req.get("stream"):
            return self._json(200, {"choices": [{"message": {"content": "Fake title"}, "finish_reason": "stop"}]})
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.end_headers()
        chunks = [{"reasoning_content": "thinking it over"}, {"content": "Echo: "}, {"content": last}]
        for delta in chunks:
            self.wfile.write(b"data: " + json.dumps({"choices": [{"delta": delta}]}).encode() + b"\n\n")
            self.wfile.flush()
        self.wfile.write(b'data: {"choices":[{"delta":{},"finish_reason":"stop"}]}\n\ndata: [DONE]\n\n')


class LiteChatTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.fake = ThreadingHTTPServer(("127.0.0.1", 0), FakeOpenAI)
        threading.Thread(target=cls.fake.serve_forever, daemon=True).start()
        cls.tmp = tempfile.TemporaryDirectory()
        cls.server = litechat.Server(data_dir=cls.tmp.name, password="test-password-1").start()
        cls.client = cls.server.client()
        cls.client.add_provider("Fake", f"http://127.0.0.1:{cls.fake.server_port}/v1")

    @classmethod
    def tearDownClass(cls):
        cls.server.stop()
        cls.fake.shutdown()
        cls.fake.server_close()
        cls.tmp.cleanup()

    def test_add_provider_is_idempotent(self):
        before = len(self.client.providers())
        again = self.client.add_provider("Fake again", f"http://127.0.0.1:{self.fake.server_port}/v1/")
        self.assertEqual(len(self.client.providers()), before)
        self.assertEqual(again["name"], "Fake")

    def test_server_url_and_port(self):
        self.assertTrue(self.server.running)
        self.assertTrue(self.server.url.startswith("http://127.0.0.1:"))
        self.assertGreater(self.server.port, 0)

    def test_models_resolve_by_id(self):
        ids = {m.id for m in self.client.models()}
        self.assertEqual(ids, {"echo", "broken"})
        key = self.client.resolve_model("echo")
        self.assertTrue(key.endswith("/echo"))
        self.assertEqual(self.client.resolve_model(key), key)
        with self.assertRaises(litechat.LiteChatError):
            self.client.resolve_model("nope")

    def test_ask(self):
        reply = self.client.ask("hello there", model="echo")
        self.assertEqual(reply.text, "Echo: hello there")
        self.assertEqual(str(reply), "Echo: hello there")
        self.assertEqual(reply.reasoning, "thinking it over")
        self.assertIn("<think>", reply.content)
        self.assertTrue(reply.message_id)

    def test_conversation_and_stream(self):
        chat = self.client.new_chat(model="echo", system_prompt="Be brief.")
        chat.send("one")
        pieces = list(stream := chat.stream("two"))
        self.assertIn("Echo: ", pieces)
        self.assertEqual(stream.reply.text, "Echo: two")
        msgs = chat.messages()
        self.assertEqual([m.role for m in msgs], ["user", "assistant", "user", "assistant"])
        self.assertEqual([m.text for m in msgs], ["one", "Echo: one", "two", "Echo: two"])
        self.assertEqual(msgs[2].parent_id, msgs[1].id)  # second turn continued the branch

        chat.rename("Renamed")
        self.assertEqual(self.client.chat(chat.id).title, "Renamed")
        self.assertIn(chat.id, [c.id for c in self.client.chats()])
        chat.delete()
        self.assertNotIn(chat.id, [c.id for c in self.client.chats()])

    def test_images(self):
        png = b"\x89PNG\r\n\x1a\n" + b"\0" * 16
        reply = self.client.ask("look", model="echo", images=[png])
        self.assertIn("look", reply.text)

    def test_reply_error(self):
        with self.assertRaises(litechat.ReplyError) as cm:
            self.client.ask("hi", model="broken")
        self.assertIn("fell over", str(cm.exception))
        self.assertIsNotNone(cm.exception.reply)

    def test_wrong_password(self):
        with self.assertRaises(litechat.AuthError):
            litechat.Client(self.server.url, "wrong-password")

    def test_export_import(self):
        self.client.ask("keep me", model="echo")
        data = self.client.export()
        before = len(self.client.chats())
        self.client.import_chats(data)
        self.assertGreater(len(self.client.chats()), before)

    def test_busy_port_fails_fast(self):
        with socket.socket() as s:
            s.bind(("127.0.0.1", 0))
            s.listen()
            with self.assertRaises(litechat.LiteChatError) as cm:
                litechat.Server(port=s.getsockname()[1], data_dir=self.tmp.name).start()
        self.assertIn("exited", str(cm.exception))


class ReplyParsingTest(unittest.TestCase):
    def test_tool_blocks(self):
        content = (
            "<think>plan</think>Looking it up.\n\n"
            '<tool name="get_time" server="clock" status="ok">{"now": "12:00"}</tool>\n\nIt is noon.'
        )
        r = litechat.Reply(content)
        self.assertEqual(r.reasoning, "plan")
        self.assertEqual(r.text, "Looking it up.\n\n\n\nIt is noon.")
        self.assertEqual(r.tool_calls, [litechat.ToolCall("get_time", "clock", True, {"now": "12:00"})])

    def test_unfinished_think(self):
        self.assertEqual(litechat.Reply("<think>still going").text, "")

    def test_binary_override(self):
        old = os.environ.get("LITECHAT_BINARY")
        os.environ["LITECHAT_BINARY"] = "/custom/litechat"
        try:
            self.assertEqual(litechat.find_binary(), "/custom/litechat")
        finally:
            if old is None:
                del os.environ["LITECHAT_BINARY"]
            else:
                os.environ["LITECHAT_BINARY"] = old


if __name__ == "__main__":
    unittest.main()
