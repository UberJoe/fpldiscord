import io
from fplutils import Utils
from teamImg import TeamImg
import discord
from discord import Option, File, Embed
from discord.ext import commands
from dotenv import load_dotenv
import pandas as pd

dotenv_path = 'config.env'
load_dotenv(dotenv_path=dotenv_path)

class FplCommands(commands.Cog):

    def __init__(self, client):
        self.client = client
        
        self.img = TeamImg()
        self.u = Utils()

        self.client.application_command(name="owner", description="Finds the owner for the specified player", cls=discord.SlashCommand)(self.owner)
        self.client.application_command(name="fixtures", description="Responds with the H2H fixtures for current or specified GW", cls=discord.SlashCommand)(self.fixtures)
        self.client.application_command(name="teamlist", description="Responds with the team of the specified owner as a list", cls=discord.SlashCommand)(self.teamlist)
        self.client.application_command(name="waivers", description="Responds with this week's waivers", cls=discord.SlashCommand)(self.waivers)
        self.client.application_command(name="dave", description="Responds with a message for whenever Dave pipes up", cls=discord.SlashCommand)(self.dave)
        self.client.application_command(name="team", description="Responds with an image of the team owned by the specified owner", cls=discord.SlashCommand)(self.team)
        self.client.application_command(name="scores", description="Get the scores of the current gameweek (live). Specify GW for previous weeks", cls=discord.SlashCommand)(self.scores)
        self.client.application_command(name="bet", description="Gets the current total goals scored for each bettor's selections", cls=discord.SlashCommand)(self.bet)

    async def owner(self, ctx, *, player_name: Option(str, description="Player's name")):
        player_df = self.u.get_player_attr(player_name, "team_x")

        if player_df.empty:
            await ctx.respond("Player not found")
            return

        response = ''
        for row in player_df.itertuples():
            if (type(row.team_x) == str):
                response += row.first_name + ' ' + row.second_name + ' is owned by: ' + row.team_x + '\n'
            else:
                response += row.first_name + ' ' + row.second_name + ' is a free agent!\n'

        embed = Embed(title=response)
        await ctx.respond(embed=embed)


    async def fixtures(self, ctx, gameweek: Option(int, description="GW to show the fixtures", max_value=38, min_value=1) = 0):
        matches_df, gameweek = self.u.get_fixtures(gameweek)

        response = ""
        spaces = matches_df["home_player"].str.len().max()
        for row in matches_df.itertuples():
            home_spaces = spaces - len(row.home_player)
            response += "```" + row.home_player + home_spaces*" " + "   vs   " + row.away_player + "```"
        embed = Embed(title="Fixtures for GW" + str(gameweek), description=response)
        await ctx.respond(embed=embed)


    async def teamlist(self, ctx, owner: Option(str, description="Team owner's first name")):
        team_df = self.u.get_team(owner)

        team_gk_df = team_df.loc[team_df['plural_name'] == 'Goalkeepers']
        team_def_df = team_df.loc[team_df['plural_name'] == 'Defenders']
        team_mid_df = team_df.loc[team_df['plural_name'] == 'Midfielders']
        team_att_df = team_df.loc[team_df['plural_name'] == 'Forwards']

        response = ""
        for row in team_gk_df.itertuples():
            response += "**" + row.web_name + "**" + " (" + row.short_name + ")\n"
        response += "\n"
        for row in team_def_df.itertuples():
            response += "**" + row.web_name + "**" + " (" + row.short_name + ")\n"
        response += "\n"
        for row in team_mid_df.itertuples():
            response += "**" + row.web_name + "**" + " (" + row.short_name + ")\n"
        response += "\n"
        for row in team_att_df.itertuples():
            response += "**" + row.web_name + "**" + " (" + row.short_name + ")\n"

        embed = Embed(title=owner+"'s Team list", description=response)
        
        await ctx.respond(embed=embed)

    
    async def waivers(self, ctx, gameweek: Option(int, description="GW to show waivers of (0 for all waivers)", max_value=38, min_value=0) = 0):
        if (gameweek == 0):
            gameweek = self.u.current_gw()
        
        transactions_df = self.u.get_transactions(gameweek)
        if transactions_df.empty:
            await ctx.respond("Couldn't find any waivers for GW" + str(gameweek))
            return

        response = ""
        for row in transactions_df.itertuples():
            response += row.short_name + ":  " + row.element_out + "  ->  " + row.element_in
            if row.result == 'a':
                response += " :white_check_mark:\n"
            else:
                response += " :x:\n"
        embed = Embed(title="Waivers in GW" + str(gameweek), description=response)
        await ctx.respond(embed=embed)


    async def dave(self, ctx):
        await ctx.respond("fuck you Dave")


    # @bot.command(description="Responds with an image of the team owned by the specified owner")
    async def team(self, ctx, owner: Option(str, description="Team owner's first name")):
        team_df = self.u.get_team(owner)
        team_image = self.img.main(team_df)
        with io.BytesIO() as image_binary:
            team_image.save(image_binary, 'PNG')
            image_binary.seek(0)
            await ctx.respond(owner + "'s team", file=File(fp=image_binary, filename='team_image.png'))


    # @bot.command(description="Get the scores of the current gameweek (live). Specify GW for previous weeks' results.")
    async def scores(self, ctx, gameweek: Option(int, description="Gameweek to get scores for", max_value=38, min_value=1) = 0):
        scores_df, gameweek = self.u.get_scores(gameweek)

        home_str_len = scores_df["home_player"].str.len().max()

        response = ""
        for row in scores_df.itertuples():
            home_spaces = home_str_len - len(row.home_player)
            match len(str(row.home_score)):
                case 1: home_score_str = "      " + str(row.home_score)
                case 2: home_score_str = "     "  + str(row.home_score)
                case 3: home_score_str = "    " + str(row.home_score)

            match len(str(row.away_score)):
                case 1: away_score_str = str(row.away_score) + "      "
                case 2: away_score_str = str(row.away_score) + "     "
                case 3: away_score_str = str(row.away_score) + "    "

            response += "```" + row.home_player + home_spaces*" " +  home_score_str + " - " + away_score_str + row.away_player + "```"
        embed = Embed(title="Live scores for GW"+str(gameweek), description=response)
        await ctx.respond(embed=embed)


    # @bot.command(description="Gets the current total goals scored for each bettor's selections")
    async def bet(self, ctx):
        await ctx.defer()
        bettors = {
            "Jack" : ["alexander-arnold", "coutinho", "cancelo", "koulibaly"], 
            "Joe" : ["diogo jota", 146, "neves", "laporte"], 
            "Harry" : ["fred", "joelinton", "cash", "keita"], 
            "Steve" : ["mount", "rodri", "sinisterra", "coady"], 
            "Hari" : ["van dijk", "grealish", "smith rowe", "watkins"], 
            "Tom" : ["cancelo", "diogo jota", "mount", "coady"]
        }

        response = ""
        
        for bettor, team in bettors.items():
            goals = 0
            for player in team:
                if type(player) is int:
                    goals_df = self.u.get_player_attr_id(player, "goals_scored")
                else:
                    goals_df = self.u.get_player_attr(player, "goals_scored")
                goals += int(goals_df["goals_scored"])
            response += "```" + bettor + ": " + str(goals) + "```"

        embed = Embed(title="Current goal totals for the bet", description=response)
        await ctx.followup.send(embed=embed)


def setup(client):
	client.add_cog(FplCommands(client))