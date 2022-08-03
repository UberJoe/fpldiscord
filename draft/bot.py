from fplutils import Utils
import discord
import os
from discord.ext import commands
import pandas as pd
# import config as cnf # uncomment if in dev env

description = 'Bot for Coq Au Ian'
intents = discord.Intents.all()
bot = discord.Bot(debug_guilds=[1001437221640994836, 1000070235954610177])
u = Utils()

fixtures = bot.create_group("fixtures", "retrieve fixtures")

@bot.command()
async def owner(ctx, *, player_name: discord.Option(str)):
    player_df = u.get_player_attr(player_name, "team_x")

    if player_df.empty:
        await ctx.respond("Player not found")
        return

    response = ''
    for row in player_df.itertuples():
        if (type(row.team_x) == str):
            response += row.first_name + ' ' + row.second_name + ' is owned by: ' + row.team_x + '\n'
        else:
            response += row.first_name + ' ' + row.second_name + ' is a free agent!\n'
    await ctx.respond(response)

@fixtures.command()
async def this_week(ctx):
    matches_df = u.get_fixtures()
    gw_df = matches_df.loc[matches_df['match'] == u.current_gw()]

    response = ""
    for row in gw_df.itertuples():
        response += row.home_player + " vs " + row.away_player + "\n"
    await ctx.respond(response)

@fixtures.command(name='gw')
async def gw_subcommand(ctx, gameweek: discord.Option(int)):
    matches_df = u.get_fixtures()
    gw_df = matches_df.loc[matches_df['match'] == gameweek]

    response = ""
    for row in gw_df.itertuples():
        response += row.home_player + " vs " + row.away_player + "\n"
    await ctx.respond(response)

@fixtures.command(name='team')
async def team_subcommand(ctx, team: discord.Option(str)):
    matches_df = u.get_fixtures()
    next5_df = matches_df.loc[matches_df['match'] < (u.current_gw() + 5)]
    team_df = next5_df.loc[(next5_df['home_player'].str.lower() == team.lower()) | (next5_df['away_player'].str.lower() == team.lower())]

    response = ""
    for row in team_df.itertuples():
        response += row.home_player + " vs " + row.away_player + "\n"
    await ctx.respond(response)

@bot.command()
async def dave(ctx):
    await ctx.respond("fuck you Dave")

bot.run(os.environ['token'])