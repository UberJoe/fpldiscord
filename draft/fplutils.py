import os
import requests
import json
import pandas as pd
import math
import unidecode
from datetime import datetime, timedelta


class Utils:

    api = {
        'transactions': 'https://draft.premierleague.com/api/draft/league/36298/transactions',
        'elements': 'https://draft.premierleague.com/api/bootstrap-static',
        'details': 'https://draft.premierleague.com/api/league/36298/details',
        'element_status': 'https://draft.premierleague.com/api/league/36298/element-status',
        'game': 'https://draft.premierleague.com/api/game',
        'live': 'https://draft.premierleague.com/api/event/{}/live',
        'entry': 'https://draft.premierleague.com/api/entry/{}/event/{}'
    }

    def __init__(self):
        self.session = requests.session()
        self.update_time = datetime.min
        self.gw_info = self.get_gw_info()
        self.data = self.update_data()

    def login(self):
        url = 'https://users.premierleague.com/accounts/login/'
        payload = {
        'password': os.getenv('PWD'),
        'login': os.getenv('EMAIL'),
        'redirect_uri': 'https://fantasy.premierleague.com/a/login',
        'app': 'plfpl-web'
        }
        self.session.post(url, data=payload)

    def update_data(self):
        now = datetime.now()
        gw_info = self.get_gw_info()

        data = {}
        if  (
                self.update_time < (now - timedelta(hours=1))
                or
                self.gw_info["current_event"] != gw_info["current_event"]
                or 
                self.gw_info["waivers_processed"] != gw_info["waivers_processed"]
            ):
                print('Calling for data from FPL API')
                data['transactions'] = self.session.get(self.api['transactions']).json()
                data['elements'] = self.session.get(self.api['elements']).json()
                data['details'] = self.session.get(self.api['details']).json()
                data['element_status'] = self.session.get(self.api['element_status']).json()
                self.gw = gw_info
                self.update_time = now
        return data

    def get_waiver_time(self, gw=0):
        if self.data is None:
            self.update_data()
        if gw == 0:
            gw = self.current_gw() - 1
        
        ts = self.data['elements']['events']['data'][gw]['waivers_time']
        return datetime.strptime(ts, "%Y-%m-%dT%H:%M:%SZ")


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

    def get_fixtures(self, gw=0):
        if gw == 0:
            gw = self.current_gw()
            if self.gw_info["current_event_finished"] == True:
                gw += 1

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
                    .drop(['finished', 'started', 'id_x', 'id_y'], axis=1)
                    .rename(columns={'event':'match',
                            'player_first_name_x': 'home_player',
                            'league_entry_1_points': 'home_score',
                            'player_first_name_y': 'away_player',
                            'league_entry_2_points': 'away_score',
                            })
                    )

        matches_df = matches_df.loc[matches_df['match'] == gw]

        return matches_df, gw

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

    def get_scores(self, gameweek=0):
        if gameweek == 0:
            gameweek = self.current_gw()
        matches_df, gameweek = self.get_fixtures(gameweek)

        url = self.api["live"].format(gameweek)
        live_data = self.session.get(url).json()
        
        for row in matches_df.itertuples():
            home_points = 0
            try:
                # get home team points
                url = self.api["entry"].format(row.entry_id_home, gameweek)
                r = self.session.get(url).json()
                picks_home_df = pd.json_normalize(r["picks"])
                
                for pick in picks_home_df.itertuples():
                    try:
                        if pick.position <= 11:
                            home_points += live_data["elements"][str(pick.element)]["stats"]["total_points"]
                    except Exception:
                        pass
            except Exception:
                pass

            away_points = 0
            try:
                # get away team points
                url = self.api["entry"].format(row.entry_id_away, gameweek)
                r = self.session.get(url).json()
                picks_away_df = pd.json_normalize(r["picks"])

                for pick in picks_away_df.itertuples():
                    try:
                        if pick.position <= 11:
                            away_points += live_data["elements"][str(pick.element)]["stats"]["total_points"]
                    except Exception:
                        pass
            except Exception:
                pass

            matches_df._set_value(row.Index, 'home_score', home_points)
            matches_df._set_value(row.Index, 'away_score', away_points)
        
        return matches_df, gameweek

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