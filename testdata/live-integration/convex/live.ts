import { query, mutation, action } from "./_generated/server";
import { v } from "convex/values";

export const listMessages = query({
  args: {
    room: v.string(),
  },
  handler: async (ctx, args) => {
    return await ctx.db
      .query("liveMessages")
      .withIndex("by_room", (q) => q.eq("room", args.room))
      .collect();
  },
});

export const sendMessage = mutation({
  args: {
    room: v.string(),
    body: v.string(),
    requestId: v.string(),
  },
  handler: async (ctx, args) => {
    const message = {
      room: args.room,
      body: args.body,
      requestId: args.requestId,
      createdAt: Date.now(),
    };
    const id = await ctx.db.insert("liveMessages", message);
    return {
      _id: id,
      ...message,
    };
  },
});

export const ping = action({
  args: {
    value: v.string(),
  },
  handler: async (_ctx, args) => {
    return {
      ok: true,
      value: args.value,
    };
  },
});
