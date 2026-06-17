import { defineSchema, defineTable } from "convex/server";
import { v } from "convex/values";

export default defineSchema({
  liveMessages: defineTable({
    room: v.string(),
    body: v.string(),
    requestId: v.string(),
    createdAt: v.float64(),
  }).index("by_room", ["room"]),
});
