import os
import requests
import json
import pandas as pd
import math
import unidecode
from datetime import datetime, timedelta, timezone

class Utils:

    api = {
        'transactions': 'https://draft.premierleague.com/api/draft/league/6/transactions',
        'elements': 'https://draft.premierleague.com/api/bootstrap-static',
        'details': 'https://draft.premierleague.com/api/league/6/details',
        'element_status': 'https://draft.premierleague.com/api/league/6/element-status',
        'game': 'https://draft.premierleague.com/api/game',
        'live': 'https://draft.premierleague.com/api/event/{}/live',
        'entry': 'https://draft.premierleague.com/api/entry/{}/event/{}'
    }

    _instance = None
    _initialized = False

    
    def __new__(cls):
        if cls._instance is None:
            cls._instance = super().__new__(cls)
            # Initialize the external API connection here
        return cls._instance


    def __init__(self):
        if not self._initialized:
            self.session = requests.session()
            self.update_time = datetime.min
            self.gw_info = self.get_gw_info()
            self.data = {}
            self.update_data()
            self._initialized = True


    def login(self):
        url = 'https://users.premierleague.com/accounts/login/'
        payload = {
        'password': os.getenv('PWD'),
        'login': os.getenv('EMAIL'),
        'redirect_uri': 'https://fantasy.premierleague.com/a/login',
        'app': 'plfpl-web'
        }
        self.session.post(url, data=payload)

    def update_data(self, force=False):
        now = datetime.now()
        gw_info = self.get_gw_info()

        data = {}
        if  (
                self.update_time < (now - timedelta(hours=1))
                or
                self.gw_info["current_event"] != gw_info["current_event"]
                or 
                self.gw_info["waivers_processed"] != gw_info["waivers_processed"]
                or 
                force == True
            ):
                print('Calling for data from FPL API')
                self.data['transactions'] = self.session.get(self.api['transactions']).json()
                self.data['elements'] = self.session.get(self.api['elements']).json()
                self.data['details'] = self.session.get(self.api['details']).json()
                self.data['element_status'] = self.session.get(self.api['element_status']).json()
                self.gw_info = gw_info
                self.update_time = now

    def get_waiver_time(self, gw=0):
        if not self.data:
            self.update_data()
        if gw == 0:
            gw = self.current_gw()
            gw_info = self.get_gw_info()
            if (gw_info["waivers_processed"] == True):
                gw = self.current_gw(True)
        
        ts = self.data['elements']['events']['data'][gw]['waivers_time']
        return datetime.strptime(ts, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)


    def get_data(self, df_name):
        self.update_data()
        
        # Dataframes from the details.json
        if df_name == 'league_entries':
            league_entry_df = pd.json_normalize(self.data['details']['league_entries'])
                
            return league_entry_df
        
        elif df_name == 'matches':
            matches_df = pd.json_normalize(self.data['details']['matches'])
                
            return matches_df
        
        elif df_name == 'standings':
            standings_df = pd.json_normalize(self.data['details']['standings'])
                
            return standings_df
        
        # Dataframes from the elements.json
        elif df_name == 'elements':
            elements_df = pd.json_normalize(self.data['elements']['elements'])
                
            return elements_df
        
        elif df_name == 'element_types':
            element_types_df = pd.json_normalize(self.data['elements']['element_types'])
                
            return element_types_df

        elif df_name == 'teams':
            elements_df = pd.json_normalize(self.data['elements']['teams'])
                
            return elements_df
        
        # Dataframes from the transactions.json
        elif df_name == 'transactions':
            transactions_df = pd.json_normalize(self.data['transactions']['transactions'])

            return transactions_df
        
        # Dataframes from the element_status.json
        elif df_name == 'element_status':
            element_status_df = pd.json_normalize(self.data['element_status']['element_status'])
                
            return element_status_df

    def get_team_players(self):
        
        # Pull the required dataframes
        element_status_df = self.get_data('element_status')
        elements_df = self.get_data('elements')
        element_types_df = self.get_data('element_types')
        league_entry_df = self.get_data('league_entries')
        teams_df = self.get_data('teams')
        
        # Built the initial player -> team dataframe
        players_df = (pd.merge(element_status_df,
                               league_entry_df,
                               how="outer",
                               left_on='owner',
                               right_on='entry_id')
                .drop(columns=[
                    'in_accepted_trade',
                    'owner',
                    'status',
                    'entry_id',
                    'entry_name',
                    'id',
                    'joined_time',
                    'player_last_name',
                    'short_name',
                    'waiver_pick'])
                .rename(columns={'player_first_name':'team_x'})
        )
        
        # Get the element details
        players_df = pd.merge(players_df, elements_df, how="outer", left_on='element', right_on='id')
        players_df = players_df[[
            'team_x',
            'element',
            'web_name',
            'first_name',
            'second_name',
            'total_points',
            'goals_scored',
            'goals_conceded',
            'clean_sheets',
            'assists',
            'bonus',
            'draft_rank',
            'element_type',
            'points_per_game',
            'red_cards',
            'yellow_cards',
            'team'
        ]]
        
        # Get the player types (GK, FWD etc.)
        players_df = (pd.merge(players_df,
                               element_types_df,
                               how='outer',
                               left_on='element_type',
                               right_on='id')
                    .drop(columns=[
                        'id',
                        'plural_name_short',
                        'singular_name',
                        'singular_name_short'])
        )

        players_df = (pd.merge(players_df, 
                               teams_df,
                               how='outer',
                               left_on='team',
                               right_on='id')
                        .drop(
                            columns=[
                            'code',
                            'id',
                            'pulse_id'
                        ])
                        .rename(columns={'name':'team_name'}))

        players_df['web_name'] = players_df['web_name'].apply(self.remove_accents)

        return players_df

    def get_player_attr(self, player: str, attr: str):
        players_df = self.get_team_players()

        if attr not in players_df.columns:
            raise ValueError(f'Attribrute \'{attr}\' is not available for players')

        selected_players_df = players_df.loc[players_df['web_name'].str.lower() == player.lower()]
        selected_players_df = selected_players_df[[
            'first_name',
            'second_name',
            attr
        ]]
        return selected_players_df
    
    def get_team_id(self, team_owner: str):
        league_entries = self.get_data("league_entries")

        team_df = league_entries[league_entries['player_first_name'].str.lower() == team_owner.lower()]

        entry_id = ""
        if not team_df.empty:
            entry_id = team_df['entry_id'].iloc[0]

        return entry_id


    def get_player_attr_id(self, player_id: int, attr: str):
        players_df = self.get_team_players()

        if attr not in players_df.columns:
            raise ValueError(f'Attribute \'{attr}\' is not available for players')
        
        selected_players_df = players_df.loc[players_df['element'] == player_id]
        selected_players_df = selected_players_df[[
            'first_name',
            'second_name',
            attr
        ]]
        return selected_players_df

    def get_team(self, owner: str, pos: str=None):
        players_df = self.get_team_players()
        team_df = players_df.loc[players_df['team_x'].str.lower() == owner.lower()]
        if pos is not None:
            team_df = team_df.loc[players_df['plural_name'].str.lower() == pos.lower()]

        if team_df.empty:
            raise ValueError(f'No players owned by: {owner}')

        return team_df
    
    def get_readable_matches(self):
        matches_df = self.get_data('matches')
        league_entry_df = self.get_data('league_entries')

        # Join to get team names and player names of entry 1 (home team)
        matches_df = (pd.merge(matches_df,
                            league_entry_df[['id', 'player_first_name', 'entry_id']],
                            how='left',
                            left_on='league_entry_1',
                            right_on='id')
                        .rename(columns={'entry_id':'entry_id_home'
                                })
                    )

        # Join to get team names and player names of entry 2 (away team)
        matches_df = (pd.merge(matches_df,
                            league_entry_df[['id', 'player_first_name', 'entry_id']],
                            how='left',
                            left_on='league_entry_2',
                            right_on='id')
                        .rename(columns={'entry_id':'entry_id_away'})
                    )

        # Drop unused columns, rename for clearer columns
        matches_df = (matches_df
                    .drop(['started', 'id_x', 'id_y'], axis=1)
                    .rename(columns={'event':'match',
                            'player_first_name_x': 'home_player',
                            'league_entry_1_points': 'home_score',
                            'player_first_name_y': 'away_player',
                            'league_entry_2_points': 'away_score',
                            })
                    )
        return matches_df

    def get_fixtures(self, gw=0):
        if gw == 0:
            gw = self.current_gw()
            if self.gw_info["current_event_finished"] == True:
                gw += 1

        matches_df = self.get_readable_matches()

        fixtures = matches_df.loc[matches_df['match'] == gw]

        return fixtures, gw
    
    def get_h2h(self, team1_id, team2_id):
        matches_df = self.get_readable_matches()
        
        h2h = matches_df.loc[
            (((matches_df['entry_id_home'] == team1_id) & (matches_df['entry_id_away'] == team2_id)) |
            ((matches_df['entry_id_away'] == team1_id) & (matches_df['entry_id_home'] == team2_id)))  &
            (matches_df['finished'] == True)
        ]

        return h2h

    def get_transactions(self, gw=0):
        transactions_df = self.get_data('transactions')
        players_df = self.get_data('elements')
        league_entries_df = self.get_data('league_entries')

        transactions_df = (pd.merge(transactions_df,
                                players_df[['web_name', 'id']],
                                how='left',
                                left_on='element_in',
                                right_on='id')
                            .drop(['element_in'], axis=1)
                            .rename(columns={'web_name':'element_in'}))

        transactions_df = (pd.merge(transactions_df,
                                players_df[['web_name', 'id']],
                                how='left',
                                left_on='element_out',
                                right_on='id')
                            .drop(['element_out', 'id_x', 'id_y', 'id'], axis=1)
                            .rename(columns={'web_name':'element_out'}))

        transactions_df = (pd.merge(transactions_df,
                                league_entries_df,
                                how='left',
                                left_on='entry',
                                right_on='entry_id')
                            .drop(['entry', 'id', 'waiver_pick', 'joined_time', 'entry_id', 'entry_name', 'priority'], axis=1)
                            .rename(columns={'index':'i'}))
        if gw != 0:
            transactions_df = transactions_df.loc[transactions_df['event'] == gw]

        transactions_df = transactions_df.sort_values(by=['event', 'i'], axis=0, ascending=True)

        return transactions_df
    
    def get_standings(self):     
        url = self.api["details"]
        details = self.session.get(url).json()
        standings = details["standings"]
        owners = details["league_entries"]

        owner_id_to_name = {owner["id"]: owner['entry_name'] for owner in owners}

        for standing in standings: 
            standing["entry_name"] = owner_id_to_name.get(standing["league_entry"], "Unknown Entry")

        return standings

    def get_scores(self, gameweek=0):
        if gameweek == 0:
            gameweek = self.current_gw()

        team_details = self.session.get(self.api["details"]).json()
        
        scores = {}
        for team in team_details['league_entries']:
            points = 0
            points += self.get_team_scores_no_bonus(team['entry_id'], gameweek)
            points += self.calculate_team_bonus(team['entry_id'], gameweek)

            scores[team['entry_id']] = {}
            scores[team['entry_id']]["team_name"] = team["entry_name"]
            scores[team['entry_id']]["points"] = points
            scores[team['entry_id']]["league_entry"] = team["id"]

        scores_by_entry = {team["league_entry"]: team for team in scores.values()}
        sorted_scores = [scores_by_entry[entry["league_entry"]] for entry in team_details["standings"] if entry["league_entry"] in scores_by_entry]
        
        return sorted_scores, gameweek
    
    def get_team_scores_no_bonus(self, team_id, gameweek=0):
        if gameweek == 0:
            gameweek = self.current_gw()

        active_team = self.get_active_team(team_id, gameweek)
        live_data = self.session.get(self.api["live"].format(gameweek)).json()

        points = 0
        for player in active_team:
            try: 
                points += live_data["elements"][str(player)]["stats"]["total_points"]
                points -= live_data["elements"][str(player)]["stats"]["bonus"]
            except Exception:
                pass

        return points
                
    
    def get_team_bonus(self, team_id, gameweek=0):
        if gameweek == 0:
            gameweek = self.current_gw()

        active_team = self.get_active_team(team_id, gameweek)

        url = self.api["live"].format(gameweek)
        fixtures = self.session.get(url).json()["fixtures"]

        bonus_points = 0
        for fixture in fixtures: 
            for stat in fixture["stats"]:
                if stat["s"] == "bonus":
                    stat_bonus = stat["a"] + stat["h"]
                    for bonus in stat_bonus:
                        if bonus["element"] in active_team: 
                            bonus_points += bonus["value"]

        return bonus_points
    
    def calculate_team_bonus(self, team_id, gameweek=0):
        if gameweek == 0:
            gameweek = self.current_gw()

        active_team = self.get_active_team(team_id, gameweek)

        url = self.api["live"].format(gameweek)
        fixtures = self.session.get(url).json()["fixtures"]

        total_bonus_points = 0  # Use a different variable for total bonus points

        for fixture in fixtures:
            if (fixture["finished_provisional"] != True):
                continue

            for stat in fixture["stats"]:
                if stat["s"] == "bps":
                    home_bps = stat["h"]
                    away_bps = stat["a"]

                    fixture_bps = home_bps + away_bps

                    sorted_bps = sorted(fixture_bps, key=lambda x: x['value'], reverse=True)
                    sorted_bps = sorted_bps[:10]

                    bonus_given = [3, 2, 1, 0]

                    # Handle ties by keeping track of the previous BPS value
                    prev_bps_value = None
                    for i, el in enumerate(sorted_bps):
                        tie = False
                        if prev_bps_value is not None and el['value'] == prev_bps_value:
                            tie = True

                        if (i > 2):
                            if (tie != True):
                                break
                            else:
                                i = 3

                        if el["element"] in active_team:
                            if (tie == True):
                                # Tied position, use the same bonus as the previous player
                                total_bonus_points += bonus_given[i - 1]
                            else:
                                total_bonus_points += bonus_given[i]
                        prev_bps_value = el['value']

        return total_bonus_points



    def get_active_team(self, team_id, gameweek=0):
        if gameweek == 0: 
            gameweek = self.current_gw()

        url = self.api["entry"].format(team_id, gameweek)
        picks = self.session.get(url).json()["picks"]

        elements = [item["element"] for item in picks if item["position"] < 12]

        return elements
    
    def get_overview(self, gameweek=0): 
        if gameweek == 0:
            gameweek = self.current_gw()

        live_url = self.api["live"].format(gameweek)
        bs_url = self.api["elements"]
        bs_data = self.session.get(bs_url).json()

        fixtures = self.session.get(live_url).json()["fixtures"]
        element_status = self.session.get(self.api["element_status"]).json()["element_status"]
        owners = self.session.get(self.api["details"]).json()["league_entries"]
        teams = bs_data["teams"]
        players = bs_data["elements"]

        # mapping
        team_id_to_name = {team["id"]: team["name"] for team in teams}
        player_id_to_name = {player["id"]: player["web_name"] for player in players}
        player_id_to_owner = {element["element"]: element["owner"] for element in element_status}
        owner_id_to_name = {owner["entry_id"]: owner['player_first_name'] for owner in owners}

        merged_fixtures = []
        for fixture in fixtures:
            merged_fixture = fixture.copy()
            merged_fixture["team_a"] = team_id_to_name.get(fixture["team_a"], "Unknown team ID: " + str(fixture["team_a"]))
            merged_fixture["team_h"] = team_id_to_name.get(fixture["team_h"], "Unknown team ID: " + str(fixture["team_h"]))
            for stat in merged_fixture["stats"]:
                for team_stat in stat["h"]:
                    player_id = team_stat["element"]
                    team_stat["name"] = player_id_to_name.get(team_stat["element"], "Unknown player ID: " + str(team_stat["element"]))
                    owner_id = player_id_to_owner.get(player_id)
                    team_stat["owner"] = owner_id
                    team_stat["owner_name"] = owner_id_to_name.get(owner_id)

                for team_stat in stat["a"]:
                    player_id = team_stat["element"]
                    team_stat["name"] = player_id_to_name.get(team_stat["element"], "Unknown player ID: " + str(team_stat["element"]))
                    owner_id = player_id_to_owner.get(player_id)
                    team_stat["owner"] = owner_id
                    team_stat["owner_name"] = owner_id_to_name.get(owner_id)

            merged_fixtures.append(merged_fixture)

        return merged_fixtures

    def current_gw(self, next_if_finished=False):
        self.update_data()

        if next_if_finished == True:
            if self.gw_info['current_event_finished'] == True:
                return self.gw_info["next_event"]

        return self.gw_info["current_event"]

    def get_entries(self):
        self.get_data('details')

    def get_gw_info(self):
        return self.session.get(self.api['game']).json()

    def remove_accents(self, string: str):
        return unidecode.unidecode(string)
    
utils = Utils()
utils.get_scores()