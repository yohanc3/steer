import * as z from "zod"

export const extraUserDataSchema = z.object({isConnectedToTelegram: z.boolean()})
export const connectionCodeSchema = z.object({code: z.string(), expires_at: z.number()})
export const BaseUserSchema = z.object({
    email: z.string(),
    name: z.string(),
    picture: z.union([z.string(), z.null()]),
    ID: z.string(),
});
export const UserSchema = BaseUserSchema.extend({
    isConnectedToTelegram: z.boolean()
})

export type extraUserData = z.infer<typeof extraUserDataSchema>
export type connectionCode = z.infer<typeof connectionCodeSchema>
export type BaseUser = z.infer<typeof BaseUserSchema>
export type User = z.infer<typeof UserSchema>
