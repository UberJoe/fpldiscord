import io
from fplutils import Utils
from teamImg import TeamImg
import discord
from discord import Option, File, Embed
from discord.ext import commands
from dotenv import load_dotenv
import json
from datetime import datetime, timezone

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
        self.client.application_command(name="update", description="Updates the data from FPL API", cls=discord.SlashCommand)(self.update)
        self.client.application_command(name="overview", description="Responds with an overview of this week's fixtures", cls=discord.SlashCommand)(self.overview)


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
            gw_info = self.u.get_gw_info()
            if (gw_info["waivers_processed"] == True):
                gameweek = self.u.current_gw(True)
        
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

        response += "\nThe next waiver deadline is: <t:" + str(int(self.u.get_waiver_time().timestamp())) + ":F>"
        
        embed = Embed(title="Waivers in GW" + str(gameweek), description=response)
        await ctx.respond(embed=embed)


    async def dave(self, ctx):
        if (str(ctx.user) == "bigsamspintofwine#0"):
            await ctx.respond("fuck you Steve")
        else:
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
        
        if (str(ctx.user) == "bigsamspintofwine#0"):
            response += "\nFuck you Steve, I hope you're losing"

        embed = Embed(title="Live scores for GW"+str(gameweek), description=response)
        await ctx.respond(embed=embed)

    async def overview(self, ctx, matches: Option(str, description="Matches to get scores for", choices=["Today's matches", "Gameweek's matches", "Live matches"])): 
        fixtures = self.u.get_overview()

        embed = discord.Embed(
            title="Gameweek fixture overview"
        )

        for fixture in fixtures:
            kickoff_time = datetime.strptime(fixture["kickoff_time"], "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)
            
            if matches == "Today's matches":
                if kickoff_time.date() != datetime.now(timezone.utc).date():
                    continue
            elif matches == "Live matches":
                if fixture["finished"] == True:
                    continue

            if kickoff_time.timestamp() > datetime.utcnow().timestamp():
                continue

            fixture_name = "**" + fixture["team_h"] + " " + str(fixture["team_h_score"]) + " - " + str(fixture["team_a_score"]) + " " + fixture["team_a"] + "**"

            stat_text = ""
            for stat in fixture["stats"]:
                emojis = {
                    "goals_scored": ":soccer: ",
                    "assists": ":regional_indicator_a: ",
                    "own_goals": "**OG** ",
                    "red_cards": ":red_square: "
                }

                all_stats = stat.get("h", []) + stat.get("a", [])

                for _stat in all_stats:
                    if stat["s"] in ["penalties_saved", "penalties_missed", "yellow_cards", "saves", "bonus", "bps"]:
                        continue
                    stat_text += emojis[stat["s"]] + _stat["name"] + " (" + str(_stat["value"]) + ")"
                    if _stat["owner_name"] is not None:
                        stat_text += "**   " + _stat["owner_name"] + "**"
                    stat_text += "\n"

            stat_text += "-- -- -- -- -- -- -- -- -- --"

            embed.add_field(
                name=fixture_name,
                value=stat_text,
                inline=False
            )

        await ctx.respond(embed=embed)


    # @bot.command(description="Gets the current total goals scored for each bettor's selections")
    async def bet(self, ctx):
        await ctx.defer()
        bettors = {
            "Harry" : [
                {"id": 206, "name": "Reece James"}, {"id": 402, "name": "Almiron"}, 
                {"id": 342, "name": "Ake"}, {"id": 506, "name": "Pedro Porro"}
            ], 
            "Joe" : [
                {"id": 108, "name": "Mbeumo"}, {"id": 349, "name": "De Bruyne"}, 
                {"id": 450, "name": "Brennan Johnson"}, {"id": 85, "name": "Solanke"}
            ], 
            "Jack" : [
                {"id": 526, "name": "Bowen"}, {"id": 55, "name": "J Ramsey"}, 
                {"id": 140, "name": "March"}, {"id": 26, "name": "Trossard"}
            ], 
            "Henry" : [
                {"id": 14, "name": "Odegaard"}, {"id": 406, "name": "Guimarães"}, 
                {"id": 304, "name": "Mac Allister"}, {"id": 365, "name": "Rodri"}
            ],
            "Steve" : [
                {"id": 599, "name": "Diaby"}, {"id": 603, "name": "Barnes"}, 
                {"id": 119, "name": "Wissa"}, {"id": 267, "name": "Andreas"}
            ], 
            "Hari" : [
                {"id": 354, "name": "Grealish"}, {"id": 504, "name": "Maddison"}, 
                {"id": 132, "name": "Ferguson"}, {"id": 290, "name": "Trent"}
            ]
        }

        response = ""
        
        for bettor, team in bettors.items():
            goals = 0
            team_str = ""
            for player in team:
                goals_df = self.u.get_player_attr_id(player["id"], "goals_scored")
                goals += goals_df["goals_scored"].item()
                team_str += "\n " + player["name"] + " (" + str(goals_df["goals_scored"].item()) + ")"
            response += "```" + bettor + ": " + str(goals) + team_str + "```"

        embed = Embed(title="Current goal totals for the bet", description=response)
        await ctx.followup.send(embed=embed)

    async def update(self, ctx):
        self.u.update_data(True)

        await ctx.respond("Data updated")


def setup(client):
	client.add_cog(FplCommands(client))